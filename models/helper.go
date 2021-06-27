// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	jsoniter "github.com/json-iterator/go"
)

func keysInt64(m map[int64]struct{}) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func valuesRepository(m map[int64]*Repository) []*Repository {
	values := make([]*Repository, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func valuesUser(m map[int64]*User) []*User {
	values := make([]*User, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// JSONUnmarshalIgnoreErroneousBOM - due to a bug in xorm (see https://gitea.com/xorm/xorm/pulls/1957) - it's
// possible that a Blob may gain an unwanted prefix of 0xff 0xfe.
func JSONUnmarshalIgnoreErroneousBOM(bs []byte, v interface{}) error {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(bs, &v)
	if err != nil && len(bs) > 2 && bs[0] == 0xff && bs[1] == 0xfe {
		err = json.Unmarshal(bs[2:], &v)
	}
	return err
}
