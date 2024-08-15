// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/log"

	elasticsearch8 "github.com/elastic/go-elasticsearch/v8"
	create8 "github.com/elastic/go-elasticsearch/v8/typedapi/indices/create"
)

// VersionedIndexName returns the full index name with version
func (i *IndexerV8) VersionedIndexName() string {
	return versionedIndexName(i.indexName, i.version)
}

func versionedIndexName(indexName string, version int) string {
	if version == 0 {
		// Old index name without version
		return indexName
	}
	return fmt.Sprintf("%s.v%d", indexName, version)
}

func (i *IndexerV8) createIndex(ctx context.Context) error {
	createIndex, err := i.Client.Indices.Create(i.VersionedIndexName()).Request(&create8.Request{
		Mappings: i.mapping,
	}).Do(ctx)
	if err != nil {
		return err
	}
	if !createIndex.Acknowledged {
		return fmt.Errorf("create index %s failed", i.VersionedIndexName())
	}

	i.checkOldIndexes(ctx)

	return nil
}

func (i *IndexerV8) initClient() (*elasticsearch8.TypedClient, error) {
	cfg := elasticsearch8.Config{
		Addresses: []string{i.url},
	}

	// logger := log.GetLogger(log.DEFAULT)

	// opts = append(opts, elastic.SetTraceLog(&log.PrintfLogger{Logf: logger.Trace}))
	// opts = append(opts, elastic.SetInfoLog(&log.PrintfLogger{Logf: logger.Info}))
	// opts = append(opts, elastic.SetErrorLog(&log.PrintfLogger{Logf: logger.Error}))

	return elasticsearch8.NewTypedClient(cfg)
}

func (i *IndexerV8) checkOldIndexes(ctx context.Context) {
	for v := 0; v < i.version; v++ {
		indexName := versionedIndexName(i.indexName, v)
		exists, err := i.Client.Indices.Exists(indexName).Do(ctx)
		if err == nil && exists {
			log.Warn("Found older elasticsearch index named %q, Gitea will keep the old NOT DELETED. You can delete the old version after the upgrade succeed.", indexName)
		}
	}
}
