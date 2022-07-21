// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !gogit

package git

import (
	"context"
)

// CacheCommit will cache the commit from the gitRepository
func (commit *Commit) CacheCommit(ctx context.Context) error {
	if commit.repo.LastCommitCache == nil {
		return nil
	}
	return commit.recursiveCache(ctx, &commit.Tree, "", 1)
}

func (commit *Commit) recursiveCache(ctx context.Context, tree *Tree, treePath string, level int) error {
	if level == 0 {
		return nil
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return err
	}

	entryPaths := make([]string, len(entries))
	for i, entry := range entries {
		entryPaths[i] = entry.Name()
	}

	_, err = WalkGitLog(ctx, commit.repo, commit, treePath, entryPaths...)
	if err != nil {
		return err
	}

	for _, treeEntry := range entries {
		// entryMap won't contain "" therefore skip this.
		if treeEntry.IsDir() {
			subTree, err := tree.SubTree(treeEntry.Name())
			if err != nil {
				return err
			}
			if err := commit.recursiveCache(ctx, subTree, treeEntry.Name(), level-1); err != nil {
				return err
			}
		}
	}

	return nil
}
