// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package system

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

// Setting is a key value store of user settings
type Setting struct {
	ID           int64  `xorm:"pk autoincr"`
	SettingKey   string `xorm:"varchar(255) unique"` // ensure key is always lowercase
	SettingValue string `xorm:"text"`
}

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return "system_setting"
}

func init() {
	db.RegisterModel(new(Setting))
}

// ErrSettingIsNotExist represents an error that a setting is not exist with special key
type ErrSettingIsNotExist struct {
	Key string
}

// Error implements error
func (err ErrSettingIsNotExist) Error() string {
	return fmt.Sprintf("System setting[%s] is not exist", err.Key)
}

// IsErrSettingIsNotExist return true if err is ErrSettingIsNotExist
func IsErrSettingIsNotExist(err error) bool {
	_, ok := err.(ErrSettingIsNotExist)
	return ok
}

// GetSetting returns specific setting
func GetSetting(key string) (*Setting, error) {
	v, err := GetSettings([]string{key})
	if err != nil {
		return nil, err
	}
	if len(v) == 0 {
		return nil, ErrSettingIsNotExist{key}
	}
	return v[key], nil
}

// GetSettings returns specific settings
func GetSettings(keys []string) (map[string]*Setting, error) {
	settings := make([]*Setting, 0, len(keys))
	if err := db.GetEngine(db.DefaultContext).
		Where(builder.In("setting_key", keys)).
		Find(&settings); err != nil {
		return nil, err
	}
	settingsMap := make(map[string]*Setting)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s
	}
	return settingsMap, nil
}

// GetAllSettings returns all settings from user
func GetAllSettings() (map[string]*Setting, error) {
	settings := make([]*Setting, 0, 5)
	if err := db.GetEngine(db.DefaultContext).
		Find(&settings); err != nil {
		return nil, err
	}
	settingsMap := make(map[string]*Setting)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s
	}
	return settingsMap, nil
}

// DeleteSetting deletes a specific setting for a user
func DeleteSetting(setting *Setting) error {
	_, err := db.GetEngine(db.DefaultContext).Delete(setting)
	return err
}

// SetSetting updates a users' setting for a specific key
func SetSetting(setting *Setting) error {
	if strings.ToLower(setting.SettingKey) != setting.SettingKey {
		return fmt.Errorf("setting key should be lowercase")
	}
	return upsertSettingValue(setting.SettingKey, setting.SettingValue)
}

func upsertSettingValue(key, value string) error {
	return db.WithTx(func(ctx context.Context) error {
		e := db.GetEngine(ctx)

		// here we use a general method to do a safe upsert for different databases (and most transaction levels)
		// 1. try to UPDATE the record and acquire the transaction write lock
		//    if UPDATE returns non-zero rows are changed, OK, the setting is saved correctly
		//    if UPDATE returns "0 rows changed", two possibilities: (a) record doesn't exist  (b) value is not changed
		// 2. do a SELECT to check if the row exists or not (we already have the transaction lock)
		// 3. if the row doesn't exist, do an INSERT (we are still protected by the transaction lock, so it's safe)
		//
		// to optimize the SELECT in step 2, we can use an extra column like `revision=revision+1`
		//    to make sure the UPDATE always returns a non-zero value for existing (unchanged) records.

		res, err := e.Exec("UPDATE system_setting SET setting_value=? WHERE setting_key=?", value, key)
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		if rows > 0 {
			// the existing row is updated, so we can return
			return nil
		}

		// in case the value isn't changed, update would return 0 rows changed, so we need this check
		has, err := e.Exist(&Setting{SettingKey: key})
		if err != nil {
			return err
		}
		if has {
			return nil
		}

		// if no existing row, insert a new row
		_, err = e.Insert(&Setting{SettingKey: key, SettingValue: value})
		return err
	})
}
