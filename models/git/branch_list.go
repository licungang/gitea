// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type BranchList []*Branch

func (branches BranchList) LoadDeletedBy(ctx context.Context) error {
	ids := container.Set[int64]{}
	for _, branch := range branches {
		ids.Add(branch.DeletedByID)
	}
	usersMap := make(map[int64]*user_model.User, len(ids))
	if err := db.GetEngine(ctx).In("id", ids.Values()).Find(&usersMap); err != nil {
		return err
	}
	for _, branch := range branches {
		branch.DeletedBy = usersMap[branch.DeletedByID]
		if branch.DeletedBy == nil {
			branch.DeletedBy = user_model.NewGhostUser()
		}
	}
	return nil
}

func (branches BranchList) LoadPusher(ctx context.Context) error {
	ids := container.Set[int64]{}
	for _, branch := range branches {
		if branch.PusherID > 0 {
			ids.Add(branch.PusherID)
		}
	}
	usersMap := make(map[int64]*user_model.User, len(ids))
	if err := db.GetEngine(ctx).In("id", ids.Values()).Find(&usersMap); err != nil {
		return err
	}
	for _, branch := range branches {
		if branch.PusherID <= 0 {
			continue
		}
		branch.Pusher = usersMap[branch.PusherID]
		if branch.Pusher == nil {
			branch.Pusher = user_model.NewGhostUser()
		}
	}
	return nil
}

const (
	BranchOrderByNameAsc        = "name ASC"
	BranchOrderByCommitTimeDesc = "commit_time DESC"
)

type FindBranchOptions struct {
	db.ListOptions
	RepoID             int64
	ExcludeBranchNames []string
	IsDeletedBranch    util.OptionalBool
	OrderBy            string
}

func (opts *FindBranchOptions) Cond() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	if len(opts.ExcludeBranchNames) > 0 {
		cond = cond.And(builder.NotIn("name", opts.ExcludeBranchNames))
	}
	if !opts.IsDeletedBranch.IsNone() {
		cond = cond.And(builder.Eq{"is_deleted": opts.IsDeletedBranch.IsTrue()})
	}
	return cond
}

func CountBranches(ctx context.Context, opts FindBranchOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.Cond()).Count(&Branch{})
}

func FindBranches(ctx context.Context, opts FindBranchOptions) (BranchList, int64, error) {
	sess := db.GetEngine(ctx).Where(opts.Cond())
	if opts.PageSize > 0 && !opts.IsListAll() {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}
	if opts.OrderBy == "" {
		opts.OrderBy = BranchOrderByCommitTimeDesc
	}
	sess = sess.OrderBy(opts.OrderBy)

	var branches []*Branch
	total, err := sess.FindAndCount(&branches)
	if err != nil {
		return nil, 0, err
	}
	return branches, total, err
}

func FindBranchNames(ctx context.Context, opts FindBranchOptions) ([]string, error) {
	sess := db.GetEngine(ctx).Select("name").Where(opts.Cond())
	if opts.PageSize > 0 && !opts.IsListAll() {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}
	if opts.OrderBy == "" {
		opts.OrderBy = BranchOrderByCommitTimeDesc
	}
	sess = sess.OrderBy(opts.OrderBy)
	var branches []string
	if err := sess.Table("branch").Find(&branches); err != nil {
		return nil, err
	}
	return branches, nil
}
