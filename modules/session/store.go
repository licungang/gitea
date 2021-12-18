// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package session

import (
	"net/http"

	"gitea.com/go-chi/session"
)

// Store represents a session store
type Store interface {
	Get(interface{}) interface{}
	Set(interface{}, interface{}) error
	Delete(interface{}) error
}

func RegenerateSession(resp http.ResponseWriter, req *http.Request) (Store, error) {
	s, err := session.RegenerateSession(resp, req)
	return s, err
}
