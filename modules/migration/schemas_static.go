// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package migration

import (
	"io"
	"io/fs"
	"path"
)

var Assets fs.FS

func openSchema(filename string) (io.ReadCloser, error) {
	return Assets.Open(path.Base(filename))
}
