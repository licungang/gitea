// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/json"

	elasticsearch8 "github.com/elastic/go-elasticsearch/v8"
	esutil8 "github.com/elastic/go-elasticsearch/v8/esutil"
	bulk8 "github.com/elastic/go-elasticsearch/v8/typedapi/core/bulk"
	some8 "github.com/elastic/go-elasticsearch/v8/typedapi/some"
	types8 "github.com/elastic/go-elasticsearch/v8/typedapi/types"
	sortorder8 "github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	textquerytype8 "github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/textquerytype"
)

const (
	issueIndexerLatestVersion = 1
)

var _ internal.Indexer = &IndexerV8{}

// Indexer implements Indexer interface
type IndexerV8 struct {
	inner                    *inner_elasticsearch.IndexerV8
	indexer_internal.Indexer // do not composite inner_elasticsearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexerV8(url, indexerName string) *IndexerV8 {
	inner := inner_elasticsearch.NewIndexerV8(url, indexerName, issueIndexerLatestVersion, defaultMappingV8)
	indexer := &IndexerV8{
		inner:   inner,
		Indexer: inner,
	}
	return indexer
}

var defaultMappingV8 = &types8.TypeMapping{
	Properties: map[string]types8.Property{
		"id":        types8.NewIntegerNumberProperty(),
		"repo_id":   types8.NewIntegerNumberProperty(),
		"is_public": types8.NewBooleanProperty(),

		"title":    types8.NewTextProperty(),
		"content":  types8.NewTextProperty(),
		"comments": types8.NewTextProperty(),

		"is_pull":              types8.NewBooleanProperty(),
		"is_closed":            types8.NewBooleanProperty(),
		"label_ids":            types8.NewIntegerNumberProperty(),
		"no_label":             types8.NewBooleanProperty(),
		"milestone_id":         types8.NewIntegerNumberProperty(),
		"project_id":           types8.NewIntegerNumberProperty(),
		"project_board_id":     types8.NewIntegerNumberProperty(),
		"poster_id":            types8.NewIntegerNumberProperty(),
		"assignee_id":          types8.NewIntegerNumberProperty(),
		"mention_ids":          types8.NewIntegerNumberProperty(),
		"reviewed_ids":         types8.NewIntegerNumberProperty(),
		"review_requested_ids": types8.NewIntegerNumberProperty(),
		"subscriber_ids":       types8.NewIntegerNumberProperty(),
		"updated_unix":         types8.NewIntegerNumberProperty(),
		"created_unix":         types8.NewIntegerNumberProperty(),
		"deadline_unix":        types8.NewIntegerNumberProperty(),
		"comment_count":        types8.NewIntegerNumberProperty(),
	},
}

// Index will save the index data
func (b *IndexerV8) Index(ctx context.Context, issues ...*internal.IndexerData) error {
	if len(issues) == 0 {
		return nil
	} else if len(issues) == 1 {
		issue := issues[0]

		raw, err := json.Marshal(issue)
		if err != nil {
			return err
		}

		_, err = b.inner.Client.Index(b.inner.VersionedIndexName()).
			Id(fmt.Sprintf("%d", issue.ID)).
			Raw(bytes.NewBuffer(raw)).
			Do(ctx)
		return err
	}

	reqs := make(bulk8.Request, 0)
	for _, issue := range issues {
		reqs = append(reqs, issue)
	}

	_, err := b.inner.Client.Bulk().
		Index(b.inner.VersionedIndexName()).
		Request(&reqs).
		Do(graceful.GetManager().HammerContext())
	return err
}

// Delete deletes indexes by ids
func (b *IndexerV8) Delete(ctx context.Context, ids ...int64) error {
	if len(ids) == 0 {
		return nil
	}
	if len(ids) == 1 {
		_, err := b.inner.Client.Delete(
			b.inner.VersionedIndexName(),
			fmt.Sprintf("%d", ids[0]),
		).Do(ctx)
		return err
	}

	bulkIndexer, err := esutil8.NewBulkIndexer(esutil8.BulkIndexerConfig{
		Client: &elasticsearch8.Client{
			BaseClient: elasticsearch8.BaseClient{
				Transport: b.inner.Client.Transport,
			},
		},
		Index: b.inner.VersionedIndexName(),
	})
	if err != nil {
		return err
	}

	for _, id := range ids {
		err = bulkIndexer.Add(ctx, esutil8.BulkIndexerItem{
			Action:     "delete",
			Index:      b.inner.VersionedIndexName(),
			DocumentID: fmt.Sprintf("%d", id),
		})
		if err != nil {
			return err
		}
	}

	return bulkIndexer.Close(ctx)
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *IndexerV8) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	query := &types8.Query{
		Bool: &types8.BoolQuery{
			Must: make([]types8.Query, 0),
		},
	}

	if options.Keyword != "" {
		searchType := &textquerytype8.Phraseprefix
		if options.IsFuzzyKeyword {
			searchType = &textquerytype8.Bestfields
		}

		query.Bool.Must = append(query.Bool.Must, types8.Query{
			MultiMatch: &types8.MultiMatchQuery{
				Query:  options.Keyword,
				Fields: []string{"title", "content", "comments"},
				Type:   searchType,
			},
		})
	}

	if len(options.RepoIDs) > 0 {
		q := types8.Query{
			Bool: &types8.BoolQuery{
				Should: make([]types8.Query, 0),
			},
		}
		if options.AllPublic {
			q.Bool.Should = append(q.Bool.Should, types8.Query{
				Term: map[string]types8.TermQuery{
					"is_public": {Value: true},
				},
			})
		}
		query.Bool.Must = append(query.Bool.Must, q)
	}

	if options.IsPull.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"is_pull": {Value: options.IsPull.Value()},
			},
		})
	}
	if options.IsClosed.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"is_closed": {Value: options.IsClosed.Value()},
			},
		})
	}

	if options.NoLabelOnly {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"no_label": {Value: true},
			},
		})
	} else {
		if len(options.IncludedLabelIDs) > 0 {
			q := types8.Query{
				Bool: &types8.BoolQuery{
					Must: make([]types8.Query, 0),
				},
			}
			for _, labelID := range options.IncludedLabelIDs {
				q.Bool.Must = append(q.Bool.Must, types8.Query{
					Term: map[string]types8.TermQuery{
						"label_ids": {Value: labelID},
					},
				})
			}
			query.Bool.Must = append(query.Bool.Must, q)
		} else if len(options.IncludedAnyLabelIDs) > 0 {
			query.Bool.Must = append(query.Bool.Must, types8.Query{
				Terms: &types8.TermsQuery{
					TermsQuery: map[string]types8.TermsQueryField{
						"label_ids": toAnySlice(options.IncludedAnyLabelIDs),
					},
				},
			})
		}
		if len(options.ExcludedLabelIDs) > 0 {
			q := types8.Query{
				Bool: &types8.BoolQuery{
					MustNot: make([]types8.Query, 0),
				},
			}
			for _, labelID := range options.ExcludedLabelIDs {
				q.Bool.MustNot = append(q.Bool.MustNot, types8.Query{
					Term: map[string]types8.TermQuery{
						"label_ids": {Value: labelID},
					},
				})
			}
			query.Bool.Must = append(query.Bool.Must, q)
		}
	}

	if len(options.MilestoneIDs) > 0 {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Terms: &types8.TermsQuery{
				TermsQuery: map[string]types8.TermsQueryField{
					"milestone_id": toAnySlice(options.MilestoneIDs),
				},
			},
		})
	}

	if options.ProjectID.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"project_id": {Value: options.ProjectID.Value()},
			},
		})
	}
	if options.ProjectColumnID.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"project_board_id": {Value: options.ProjectColumnID.Value()},
			},
		})
	}

	if options.PosterID.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"poster_id": {Value: options.PosterID.Value()},
			},
		})
	}

	if options.AssigneeID.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"assignee_id": {Value: options.AssigneeID.Value()},
			},
		})
	}

	if options.MentionID.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"mention_ids": {Value: options.MentionID.Value()},
			},
		})
	}

	if options.ReviewedID.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"reviewed_ids": {Value: options.ReviewedID.Value()},
			},
		})
	}

	if options.ReviewRequestedID.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"review_requested_ids": {Value: options.ReviewRequestedID.Value()},
			},
		})
	}

	if options.SubscriberID.Has() {
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Term: map[string]types8.TermQuery{
				"subscriber_ids": {Value: options.SubscriberID.Value()},
			},
		})
	}

	if options.UpdatedAfterUnix.Has() || options.UpdatedBeforeUnix.Has() {
		rangeQuery := types8.NumberRangeQuery{}
		if options.UpdatedAfterUnix.Has() {
			rangeQuery.Gte = some8.Float64(float64(options.UpdatedAfterUnix.Value()))
		}
		if options.UpdatedBeforeUnix.Has() {
			rangeQuery.Lte = some8.Float64(float64(options.UpdatedBeforeUnix.Value()))
		}
		query.Bool.Must = append(query.Bool.Must, types8.Query{
			Range: map[string]types8.RangeQuery{
				"updated_unix": rangeQuery,
			},
		})
	}

	if options.SortBy == "" {
		options.SortBy = internal.SortByCreatedAsc
	}
	field, fieldSort := parseSortByV8(options.SortBy)
	sort := []types8.SortCombinations{
		&types8.SortOptions{SortOptions: map[string]types8.FieldSort{
			field: fieldSort,
			"id":  {Order: &sortorder8.Desc},
		}},
	}

	// See https://stackoverflow.com/questions/35206409/elasticsearch-2-1-result-window-is-too-large-index-max-result-window/35221900
	// TODO: make it configurable since it's configurable in elasticsearch
	const maxPageSize = 10000

	skip, limit := indexer_internal.ParsePaginator(options.Paginator, maxPageSize)
	searchResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Query(query).
		Sort(sort...).
		From(skip).Size(limit).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	hits := make([]internal.Match, 0, limit)
	for _, hit := range searchResult.Hits.Hits {
		id, _ := strconv.ParseInt(*hit.Id_, 10, 64)
		hits = append(hits, internal.Match{
			ID: id,
		})
	}

	return &internal.SearchResult{
		Total: searchResult.Hits.Total.Value,
		Hits:  hits,
	}, nil
}

func parseSortByV8(sortBy internal.SortBy) (string, types8.FieldSort) {
	field := strings.TrimPrefix(string(sortBy), "-")
	sort := types8.FieldSort{
		Order: &sortorder8.Asc,
	}
	if strings.HasPrefix(string(sortBy), "-") {
		sort.Order = &sortorder8.Desc
	}

	return field, sort
}
