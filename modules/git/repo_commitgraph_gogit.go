// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"os"
	"path"

	gitealog "code.gitea.io/gitea/modules/log"

	commitgraph "github.com/go-git/go-git/v5/plumbing/format/commitgraph/v2"
	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

// CommitNodeIndex returns the index for walking commit graph
func (r *Repository) CommitNodeIndex() (cgobject.CommitNodeIndex, *os.File) {
	indexPath := path.Join(r.Path, "objects", "info", "commit-graph")

	file, err := os.Open(indexPath)
	if err == nil {
		index, err := commitgraph.OpenFileIndex(file)
		if err == nil {
			return cgobject.NewGraphCommitNodeIndex(index, r.gogitRepo.Storer), file
		}
	}

	if !os.IsNotExist(err) {
		gitealog.Warn("Unable to read commit-graph for %s: %v", r.Path, err)
	}

	return cgobject.NewObjectCommitNodeIndex(r.gogitRepo.Storer), nil
}
