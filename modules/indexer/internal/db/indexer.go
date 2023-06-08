// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"code.gitea.io/gitea/modules/indexer/internal"
)

var _ internal.Indexer = &Indexer{}

// Indexer represents a basic db indexer implementation
type Indexer struct{}

// Init initializes the indexer
func (i *Indexer) Init() (bool, error) {
	// nothing to do
	return false, nil
}

// Ping checks if the indexer is available
func (i *Indexer) Ping() bool {
	// No need to ping database to check if it is available.
	// If the database goes down, Gitea will go down, so nobody will care if the indexer is available.
	return true
}

// Close closes the indexer
func (i *Indexer) Close() {
	// nothing to do
}
