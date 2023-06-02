// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"reflect"
)

// Storage represents configuration of storages
type Storage struct {
	Type        string
	Path        string
	Section     ConfigSection
	ServeDirect bool
}

// MapTo implements the Mappable interface
func (s *Storage) MapTo(v interface{}) error {
	pathValue := reflect.ValueOf(v).Elem().FieldByName("Path")
	if pathValue.IsValid() && pathValue.Kind() == reflect.String {
		pathValue.SetString(s.Path)
	}
	if s.Section != nil {
		return s.Section.MapTo(v)
	}
	return nil
}

const storageSectionName = "storage"

func getStorageSection(rootCfg ConfigProvider) ConfigSection {
	storageSec := rootCfg.Section(storageSectionName)
	// Global Defaults
	storageSec.Key("MINIO_ENDPOINT").MustString("localhost:9000")
	storageSec.Key("MINIO_ACCESS_KEY_ID").MustString("")
	storageSec.Key("MINIO_SECRET_ACCESS_KEY").MustString("")
	storageSec.Key("MINIO_BUCKET").MustString("gitea")
	storageSec.Key("MINIO_LOCATION").MustString("us-east-1")
	storageSec.Key("MINIO_USE_SSL").MustBool(false)
	storageSec.Key("MINIO_INSECURE_SKIP_VERIFY").MustBool(false)
	storageSec.Key("MINIO_CHECKSUM_ALGORITHM").MustString("default")
	return storageSec
}

func getStorage(rootCfg ConfigProvider, name, typ string, targetSec ConfigSection) *Storage {
	sec := getStorageSection(rootCfg)

	if targetSec == nil {
		targetSec, _ = rootCfg.NewSection(name)
	}

	var storage Storage
	storage.Section = targetSec
	storage.Type = typ

	overrides := make([]ConfigSection, 0, 3)
	nameSec, err := rootCfg.GetSection(storageSectionName + "." + name)
	if err == nil {
		overrides = append(overrides, nameSec)
	}

	typeSec, err := rootCfg.GetSection(storageSectionName + "." + typ)
	if err == nil {
		overrides = append(overrides, typeSec)
		nextType := typeSec.Key("STORAGE_TYPE").String()
		if len(nextType) > 0 {
			storage.Type = nextType // Support custom STORAGE_TYPE
		}
	}
	overrides = append(overrides, sec)

	for _, override := range overrides {
		for _, key := range override.Keys() {
			if !targetSec.HasKey(key.Name()) {
				_, _ = targetSec.NewKey(key.Name(), key.Value())
			}
		}
		if len(storage.Type) == 0 {
			storage.Type = override.Key("STORAGE_TYPE").String()
		}
	}
	storage.ServeDirect = storage.Section.Key("SERVE_DIRECT").MustBool(false)

	// Specific defaults
	storage.Path = storage.Section.Key("PATH").MustString(filepath.Join(AppDataPath, name))
	if !filepath.IsAbs(storage.Path) {
		storage.Path = filepath.Join(AppWorkPath, storage.Path)
		storage.Section.Key("PATH").SetValue(storage.Path)
	}
	storage.Section.Key("MINIO_BASE_PATH").MustString(name + "/")

	return &storage
}
