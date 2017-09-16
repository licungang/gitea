// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"github.com/go-xorm/builder"
)

// RepositoryList contains a list of repositories
type RepositoryList []*Repository

// RepositoryListOfMap make list from values of map
func RepositoryListOfMap(repoMap map[int64]*Repository) RepositoryList {
	return RepositoryList(valuesRepository(repoMap))
}

func (repos RepositoryList) loadAttributes(e Engine) error {
	if len(repos) == 0 {
		return nil
	}

	// Load owners.
	set := make(map[int64]struct{})
	for i := range repos {
		set[repos[i].OwnerID] = struct{}{}
	}
	users := make(map[int64]*User, len(set))
	if err := e.
		Where("id > 0").
		In("id", keysInt64(set)).
		Find(&users); err != nil {
		return fmt.Errorf("find users: %v", err)
	}
	for i := range repos {
		repos[i].Owner = users[repos[i].OwnerID]
	}
	return nil
}

// LoadAttributes loads the attributes for the given RepositoryList
func (repos RepositoryList) LoadAttributes() error {
	return repos.loadAttributes(x)
}

// MirrorRepositoryList contains the mirror repositories
type MirrorRepositoryList []*Repository

func (repos MirrorRepositoryList) loadAttributes(e Engine) error {
	if len(repos) == 0 {
		return nil
	}

	// Load mirrors.
	repoIDs := make([]int64, 0, len(repos))
	for i := range repos {
		if !repos[i].IsMirror {
			continue
		}

		repoIDs = append(repoIDs, repos[i].ID)
	}
	mirrors := make([]*Mirror, 0, len(repoIDs))
	if err := e.
		Where("id > 0").
		In("repo_id", repoIDs).
		Find(&mirrors); err != nil {
		return fmt.Errorf("find mirrors: %v", err)
	}

	set := make(map[int64]*Mirror)
	for i := range mirrors {
		set[mirrors[i].RepoID] = mirrors[i]
	}
	for i := range repos {
		repos[i].Mirror = set[repos[i].ID]
	}
	return nil
}

// LoadAttributes loads the attributes for the given MirrorRepositoryList
func (repos MirrorRepositoryList) LoadAttributes() error {
	return repos.loadAttributes(x)
}

// SearchRepoOptions holds the search options
// swagger:parameters repoSearch
type SearchRepoOptions struct {
	// Keyword to search
	//
	// in: query
	Keyword string `json:"q"`
	// Owner in we search search
	//
	// in: query
	OwnerID     int64         `json:"uid"`
	OrderBy     SearchOrderBy `json:"-"`
	Private     bool          `json:"-"` // Include private repositories in results
	Collaborate bool          `json:"-"` // Include collaborative repositories
	Starred     bool          `json:"-"`
	Page        int           `json:"-"`
	IsProfile   bool          `json:"-"`
	// Limit of result
	//
	// maximum: setting.ExplorePagingNum
	// in: query
	PageSize int `json:"limit"` // Can be smaller than or equal to setting.ExplorePagingNum
	// Type of repository to search (related to owner if present)
	//
	// in: query
	RepoType RepoType `json:"type"`
}

// RepoType is repository filtering type identifier
type RepoType string

const (
	// RepoTypeAny any type (default)
	RepoTypeAny RepoType = ""
	// RepoTypeFork fork type
	RepoTypeFork = "FORK"
	// RepoTypeMirror mirror type
	RepoTypeMirror = "MIRROR"
	// RepoTypeSource source type
	RepoTypeSource = "SOURCE"
	// RepoTypeCollaborative collaborative type
	RepoTypeCollaborative = "COLLABORATIVE"
)

//SearchOrderBy is used to sort the result
type SearchOrderBy string

func (s SearchOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchOrderByAlphabeticallyReverse               = "name DESC"
	SearchOrderByLeastUpdated                        = "updated_unix ASC"
	SearchOrderByRecentUpdated                       = "updated_unix DESC"
	SearchOrderByOldest                              = "created_unix ASC"
	SearchOrderByNewest                              = "created_unix DESC"
	SearchOrderBySize                                = "size ASC"
	SearchOrderBySizeReverse                         = "size DESC"
)

// SearchRepositoryByName takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryByName(opts *SearchRepoOptions) (repos RepositoryList, _ int64, _ error) {
	// Check if user with Owner ID exists
	if opts.OwnerID > 0 {
		userExists, err := GetUser(&User{ID: opts.OwnerID})
		if err != nil {
			return nil, 0, err
		}
		if !userExists {
			return nil, 0, ErrUserNotExist{UID: opts.OwnerID}
		}
	}

	// Check and set page to correct number
	if opts.Page <= 0 {
		opts.Page = 1
	}

	var cond = builder.NewCond()

	// Add repository name keyword to search for
	if opts.Keyword != "" {
		opts.Keyword = strings.ToLower(opts.Keyword)
		cond = cond.And(builder.Like{"lower_name", opts.Keyword})
	}

	// Exclude private repositories
	if !opts.Private {
		cond = cond.And(builder.Eq{"is_private": false})
	}

	includeStarred := false
	if opts.OwnerID > 0 {
		if opts.Starred {
			// Return only starred repositories by Owner
			includeStarred = true
			cond = builder.Eq{
				"star.uid": opts.OwnerID,
			}
		} else {
			// Set user access conditions
			var accessCond builder.Cond = builder.NewCond()

			// Add Owner ID to access conditions
			if opts.RepoType != RepoTypeCollaborative {
				accessCond = accessCond.Or(builder.Eq{"owner_id": opts.OwnerID})
			}

			// Include collaborative repositories
			if opts.Collaborate && (opts.RepoType == RepoTypeAny || opts.RepoType == RepoTypeMirror || opts.RepoType == RepoTypeCollaborative) {
				// Add repositories where user is set as collaborator directly
				accessCond = accessCond.Or(builder.And(
					builder.Expr("id IN (SELECT repo_id FROM `access` WHERE access.user_id = ?)", opts.OwnerID),
					builder.Neq{"owner_id": opts.OwnerID}))
			}

			// Add user access conditions to search
			cond = cond.And(accessCond)
		}
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = SearchOrderByAlphabetically
	}

	// Add general filters for repository
	if opts.RepoType != RepoTypeAny {
		cond = cond.And(builder.Eq{"is_mirror": opts.RepoType == RepoTypeMirror})

		switch opts.RepoType {
		case RepoTypeFork:
			cond = cond.And(builder.Eq{"is_fork": true})
		case RepoTypeSource:
			cond = cond.And(builder.Eq{"is_fork": false})
		}
	}

	sess := x.NewSession()
	defer sess.Close()

	if includeStarred {
		sess.Join("INNER", "star", "star.repo_id = repository.id")
	}

	count, err := sess.
		Where(cond).
		Count(new(Repository))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	// Set again after reset by Count()
	if includeStarred {
		sess.Join("INNER", "star", "star.repo_id = repository.id")
	}

	repos = make([]*Repository, 0, opts.PageSize)
	if err = sess.
		Where(cond).
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		OrderBy(opts.OrderBy.String()).
		Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if !opts.IsProfile {
		if err = repos.loadAttributes(sess); err != nil {
			return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
		}
	}

	return repos, count, nil
}

// Repositories returns all repositories
func Repositories(opts *SearchRepoOptions) (_ RepositoryList, count int64, err error) {
	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "id ASC"
	}

	repos := make(RepositoryList, 0, opts.PageSize)

	if err = x.
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		OrderBy(opts.OrderBy.String()).
		Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if err = repos.loadAttributes(x); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
	}

	count = countRepositories(-1, opts.Private)

	return repos, count, nil
}
