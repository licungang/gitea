// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

type SpecList []*ActionScheduleSpec

func (specs SpecList) GetScheduleIDs() []int64 {
	ids := make(container.Set[int64], len(specs))
	for _, spec := range specs {
		ids.Add(spec.ScheduleID)
	}
	return ids.Values()
}

func (specs SpecList) LoadSchedules() error {
	scheduleIDs := specs.GetScheduleIDs()
	schedules, err := GetSchedulesMapByIDs(scheduleIDs)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		spec.Schedule = schedules[spec.ScheduleID]
	}
	return nil
}

func (specs SpecList) GetRepoIDs() []int64 {
	ids := make(container.Set[int64], len(specs))
	for _, spec := range specs {
		ids.Add(spec.RepoID)
	}
	return ids.Values()
}

func (specs SpecList) LoadRepos() error {
	repoIDs := specs.GetRepoIDs()
	repos, err := repo_model.GetRepositoriesMapByIDs(repoIDs)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		spec.Repo = repos[spec.RepoID]
	}
	return nil
}

type FindSpecOptions struct {
	db.ListOptions
	RepoID int64
}

func (opts FindSpecOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	return cond
}

// FinAllSpecs retrieves all specs that match the given options.
// It retrieves the specs in pages of size 50 and appends them to a list until all specs have been retrieved.
// It also loads the schedules for each spec.
func FinAllSpecs(ctx context.Context, opts FindSpecOptions) (SpecList, error) {
	// Set the page size and initialize the list of all specs
	pageSize := 50
	var allSpecs SpecList

	// Retrieve specs in pages until all specs have been retrieved
	for page := 1; ; page++ {
		// Create a new query engine and apply the given conditions
		e := db.GetEngine(ctx).Where(opts.toConds())

		// Limit the results to the current page
		e.Limit(pageSize, (page-1)*pageSize)

		// Retrieve the specs for the current page and add them to the list of all specs
		var specs SpecList
		total, err := e.Desc("id").FindAndCount(&specs)
		if err != nil {
			break
		}
		allSpecs = append(allSpecs, specs...)

		// Stop if all specs have been retrieved
		if int(total) < pageSize {
			break
		}
	}

	// Load the schedules for each spec
	if err := allSpecs.LoadSchedules(); err != nil {
		return nil, err
	}

	// Return the list of all specs
	return allSpecs, nil
}

func FindSpecs(ctx context.Context, opts FindSpecOptions) (SpecList, int64, error) {
	e := db.GetEngine(ctx).Where(opts.toConds())
	if opts.PageSize > 0 && opts.Page >= 1 {
		e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	var specs SpecList
	total, err := e.Desc("id").FindAndCount(&specs)
	if err != nil {
		return nil, 0, err
	}

	if err := specs.LoadSchedules(); err != nil {
		return nil, 0, err
	}
	return specs, total, nil
}

func CountSpecs(ctx context.Context, opts FindSpecOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds()).Count(new(ActionScheduleSpec))
}
