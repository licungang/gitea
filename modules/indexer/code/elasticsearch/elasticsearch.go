// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/typesniffer"

	"github.com/go-enry/go-enry/v2"
	"github.com/olivere/elastic/v7"
)

const (
	esRepoIndexerLatestVersion = 2
	// multi-match-types, currently only 2 types are used
	// Reference: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-multi-match-query.html#multi-match-types
	esMultiMatchTypeBestFields   = "best_fields"
	esMultiMatchTypePhrasePrefix = "phrase_prefix"
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	inner                    *inner_elasticsearch.Indexer
	indexer_internal.Indexer // do not composite inner_elasticsearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string) *Indexer {
	inner := inner_elasticsearch.NewIndexer(url, indexerName, esRepoIndexerLatestVersion, defaultMapping)
	indexer := &Indexer{
		inner:   inner,
		Indexer: inner,
	}
	return indexer
}

const (
	defaultMapping = `{
		"mappings": {
			"properties": {
				"repo_id": {
					"type": "long",
					"index": true
				},
				"is_wiki": {
					"type": "boolean"
					"index": true
				}
				"content": {
					"type": "text",
					"term_vector": "with_positions_offsets",
					"index": true
				},
				"commit_id": {
					"type": "keyword",
					"index": true
				},
				"language": {
					"type": "keyword",
					"index": true
				},
				"updated_at": {
					"type": "long",
					"index": true
				}
			}
		}
	}`
)

func (b *Indexer) addUpdate(ctx context.Context, batchWriter git.WriteCloserError, batchReader *bufio.Reader, sha string, update internal.FileUpdate, repo *repo_model.Repository, isWiki bool) ([]elastic.BulkableRequest, error) {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return nil, nil
	}

	size := update.Size
	repoPath := repo.RepoPath()
	if isWiki {
		repoPath = repo.WikiPath()
	}

	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = git.NewCommand(ctx, "cat-file", "-s").AddDynamicArguments(update.BlobSha).RunStdString(&git.RunOpts{Dir: repoPath})
		if err != nil {
			return nil, err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return nil, fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}

	if size > setting.Indexer.MaxIndexerFileSize {
		return []elastic.BulkableRequest{b.addDelete(update.Filename, repo, isWiki)}, nil
	}

	if _, err := batchWriter.Write([]byte(update.BlobSha + "\n")); err != nil {
		return nil, err
	}

	_, _, size, err = git.ReadBatchLine(batchReader)
	if err != nil {
		return nil, err
	}

	fileContents, err := io.ReadAll(io.LimitReader(batchReader, size))
	if err != nil {
		return nil, err
	} else if !typesniffer.DetectContentType(fileContents).IsText() {
		// FIXME: UTF-16 files will probably fail here
		return nil, nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return nil, err
	}
	id := internal.FilenameIndexerID(repo.ID, isWiki, update.Filename)

	return []elastic.BulkableRequest{
		elastic.NewBulkIndexRequest().
			Index(b.inner.VersionedIndexName()).
			Id(id).
			Doc(map[string]any{
				"repo_id":    repo.ID,
				"is_wiki":    isWiki,
				"content":    string(charset.ToUTF8DropErrors(fileContents, charset.ConvertOpts{})),
				"commit_id":  sha,
				"language":   analyze.GetCodeLanguage(update.Filename, fileContents),
				"updated_at": timeutil.TimeStampNow(),
			}),
	}, nil
}

func (b *Indexer) addDelete(filename string, repo *repo_model.Repository, isWiki bool) elastic.BulkableRequest {
	id := internal.FilenameIndexerID(repo.ID, isWiki, filename)
	return elastic.NewBulkDeleteRequest().
		Index(b.inner.VersionedIndexName()).
		Id(id)
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, isWiki bool, sha string, changes *internal.RepoChanges) error {
	repoPath := repo.RepoPath()
	if isWiki {
		repoPath = repo.WikiPath()
	}

	reqs := make([]elastic.BulkableRequest, 0)
	if len(changes.Updates) > 0 {
		// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
		if err := git.EnsureValidGitRepository(ctx, repoPath); err != nil {
			log.Error("Unable to open git repo: %s for %-v: %v", repoPath, repo, err)
			return err
		}

		batchWriter, batchReader, cancel := git.CatFileBatch(ctx, repoPath)
		defer cancel()

		for _, update := range changes.Updates {
			updateReqs, err := b.addUpdate(ctx, batchWriter, batchReader, sha, update, repo, isWiki)
			if err != nil {
				return err
			}
			if len(updateReqs) > 0 {
				reqs = append(reqs, updateReqs...)
			}
		}
		cancel()
	}

	for _, filename := range changes.RemovedFilenames {
		reqs = append(reqs, b.addDelete(filename, repo, isWiki))
	}

	if len(reqs) > 0 {
		esBatchSize := 50

		for i := 0; i < len(reqs); i += esBatchSize {
			_, err := b.inner.Client.Bulk().
				Index(b.inner.VersionedIndexName()).
				Add(reqs[i:min(i+esBatchSize, len(reqs))]...).
				Do(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Delete deletes indexes by ids
func (b *Indexer) Delete(ctx context.Context, repoID int64) error {
	_, err := b.inner.Client.DeleteByQuery(b.inner.VersionedIndexName()).
		Query(elastic.NewTermsQuery("repo_id", repoID)).
		Do(ctx)
	return err
}

// indexPos find words positions for start and the following end on content. It will
// return the beginning position of the first start and the ending position of the
// first end following the start string.
// If not found any of the positions, it will return -1, -1.
func indexPos(content, start, end string) (int, int) {
	startIdx := strings.Index(content, start)
	if startIdx < 0 {
		return -1, -1
	}
	endIdx := strings.Index(content[startIdx+len(start):], end)
	if endIdx < 0 {
		return -1, -1
	}
	return startIdx, startIdx + len(start) + endIdx + len(end)
}

func convertResult(searchResult *elastic.SearchResult, kw string, pageSize int) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	hits := make([]*internal.SearchResult, 0, pageSize)
	for _, hit := range searchResult.Hits.Hits {
		// FIXME: There is no way to get the position the keyword on the content currently on the same request.
		// So we get it from content, this may made the query slower. See
		// https://discuss.elastic.co/t/fetching-position-of-keyword-in-matched-document/94291
		var startIndex, endIndex int
		c, ok := hit.Highlight["content"]
		if ok && len(c) > 0 {
			// FIXME: Since the highlighting content will include <em> and </em> for the keywords,
			// now we should find the positions. But how to avoid html content which contains the
			// <em> and </em> tags? If elastic search has handled that?
			startIndex, endIndex = indexPos(c[0], "<em>", "</em>")
			if startIndex == -1 {
				panic(fmt.Sprintf("1===%s,,,%#v,,,%s", kw, hit.Highlight, c[0]))
			}
		} else {
			panic(fmt.Sprintf("2===%#v", hit.Highlight))
		}

		repoID, isWiki, fileName, err := internal.ParseIndexerID(hit.Id)
		if err != nil {
			return 0, nil, nil, err
		}

		res := make(map[string]any)
		if err := json.Unmarshal(hit.Source, &res); err != nil {
			return 0, nil, nil, err
		}

		language := res["language"].(string)

		hits = append(hits, &internal.SearchResult{
			RepoID:      repoID,
			IsWiki:      isWiki,
			Filename:    fileName,
			CommitID:    res["commit_id"].(string),
			Content:     res["content"].(string),
			UpdatedUnix: timeutil.TimeStamp(res["updated_at"].(float64)),
			Language:    language,
			StartIndex:  startIndex,
			EndIndex:    endIndex - 9, // remove the length <em></em> since we give Content the original data
			Color:       enry.GetColor(language),
		})
	}

	return searchResult.TotalHits(), hits, extractAggs(searchResult), nil
}

func extractAggs(searchResult *elastic.SearchResult) []*internal.SearchResultLanguages {
	var searchResultLanguages []*internal.SearchResultLanguages
	agg, found := searchResult.Aggregations.Terms("language")
	if found {
		searchResultLanguages = make([]*internal.SearchResultLanguages, 0, 10)

		for _, bucket := range agg.Buckets {
			searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
				Language: bucket.Key.(string),
				Color:    enry.GetColor(bucket.Key.(string)),
				Count:    int(bucket.DocCount),
			})
		}
	}
	return searchResultLanguages
}

// Search searches for codes and language stats by given conditions.
func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	searchType := esMultiMatchTypePhrasePrefix
	if opts.IsKeywordFuzzy {
		searchType = esMultiMatchTypeBestFields
	}

	kwQuery := elastic.NewMultiMatchQuery(opts.Keyword, "content").Type(searchType)
	query := elastic.NewBoolQuery()
	query = query.Must(kwQuery)
	if len(opts.RepoIDs) > 0 {
		repoStrs := make([]any, 0, len(opts.RepoIDs))
		for _, repoID := range opts.RepoIDs {
			repoStrs = append(repoStrs, repoID)
		}
		repoQuery := elastic.NewTermsQuery("repo_id", repoStrs...)
		query = query.Must(repoQuery)
	}

	opts.IsWiki = optional.Some(false) // TODO

	if opts.IsWiki.Has() {
		query = query.Must(elastic.NewTermQuery("is_wiki", opts.IsWiki.Value()))
	}

	var (
		start, pageSize = opts.GetSkipTake()
		kw              = "<em>" + opts.Keyword + "</em>"
		aggregation     = elastic.NewTermsAggregation().Field("language").Size(10).OrderByCountDesc()
	)

	if len(opts.Language) == 0 {
		searchResult, err := b.inner.Client.Search().
			Index(b.inner.VersionedIndexName()).
			Aggregation("language", aggregation).
			Query(query).
			Highlight(
				elastic.NewHighlight().
					Field("content").
					NumOfFragments(0). // return all highting content on fragments
					HighlighterType("fvh"),
			).
			Sort("repo_id", true).
			From(start).Size(pageSize).
			Do(ctx)
		if err != nil {
			return 0, nil, nil, err
		}

		return convertResult(searchResult, kw, pageSize)
	}

	langQuery := elastic.NewMatchQuery("language", opts.Language)
	countResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Aggregation("language", aggregation).
		Query(query).
		Size(0). // We only need stats information
		Do(ctx)
	if err != nil {
		return 0, nil, nil, err
	}

	query = query.Must(langQuery)
	searchResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Query(query).
		Highlight(
			elastic.NewHighlight().
				Field("content").
				NumOfFragments(0). // return all highting content on fragments
				HighlighterType("fvh"),
		).
		Sort("repo_id", true).
		From(start).Size(pageSize).
		Do(ctx)
	if err != nil {
		return 0, nil, nil, err
	}

	total, hits, _, err := convertResult(searchResult, kw, pageSize)

	return total, hits, extractAggs(countResult), err
}
