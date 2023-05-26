// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RepoArchiveStorage(t *testing.T) {
	iniStr := `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage]
STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadRepoArchive(cfg))
	storage := RepoArchive.Storage

	assert.EqualValues(t, "minio", storage.Type)
	assert.EqualValues(t, "gitea", storage.Section.Key("MINIO_BUCKET").String())

	iniStr = `
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
[storage.repo-archive]
STORAGE_TYPE = s3

[storage.s3]
STORAGE_TYPE            = minio
MINIO_ENDPOINT          = s3.my-domain.net
MINIO_BUCKET            = gitea
MINIO_LOCATION          = homenet
MINIO_USE_SSL           = true
MINIO_ACCESS_KEY_ID     = correct_key
MINIO_SECRET_ACCESS_KEY = correct_key
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)

	assert.NoError(t, loadRepoArchive(cfg))
	storage = RepoArchive.Storage

	assert.EqualValues(t, "minio", storage.Type)
	assert.EqualValues(t, "gitea", storage.Section.Key("MINIO_BUCKET").String())
}
