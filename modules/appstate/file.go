// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package appstate

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/json"
)

// FileStore can be used to store app state items in local filesystem
type FileStore struct {
	path string
}

func (f *FileStore) genFilePath(item StateItem) string {
	return filepath.Join(f.path, item.Name())
}

// Get reads the state item
func (f *FileStore) Get(item StateItem) error {
	b, e := ioutil.ReadFile(f.genFilePath(item))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	e = json.Unmarshal(b, item)
	return e
}

// Set saves the state item
func (f *FileStore) Set(item StateItem) error {
	b, e := json.Marshal(item)
	if e != nil {
		return e
	}
	return ioutil.WriteFile(f.genFilePath(item), b, fs.FileMode(0644))
}

// NewFileStore returns a new file store
func NewFileStore(path string) (*FileStore, error) {
	_ = os.Mkdir(path, fs.FileMode(0755))
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return &FileStore{path: path}, nil
}
