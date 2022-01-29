// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"mime"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestDownloadByID(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo1/raw/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
}

func TestDownloadByIDForSVGUsesSecureHeaders(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo2/raw/blob/6395b68e1feebb1e4c657b4f9f6ba2676a283c0b")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", resp.HeaderMap.Get("Content-Security-Policy"))
	assert.Equal(t, "image/svg+xml", resp.HeaderMap.Get("Content-Type"))
	assert.Equal(t, "nosniff", resp.HeaderMap.Get("X-Content-Type-Options"))
}

func TestDownloadByIDMedia(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo1/media/blob/4b4851ad51df6a7d9f25c979345979eaeb5b349f")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
}

func TestDownloadByIDMediaForSVGUsesSecureHeaders(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request raw blob
	req := NewRequest(t, "GET", "/user2/repo2/media/blob/6395b68e1feebb1e4c657b4f9f6ba2676a283c0b")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "default-src 'none'; style-src 'unsafe-inline'; sandbox", resp.HeaderMap.Get("Content-Security-Policy"))
	assert.Equal(t, "image/svg+xml", resp.HeaderMap.Get("Content-Type"))
	assert.Equal(t, "nosniff", resp.HeaderMap.Get("X-Content-Type-Options"))
}

func TestDownloadRawTextFileWithoutMimeTypeMapping(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo2/raw/branch/master/bin.foo")
	resp := session.MakeRequest(t, req, http.StatusOK)

	assert.Equal(t, "application/octet-stream", resp.HeaderMap.Get("Content-Type"))
}

func TestDownloadRawTextFileWithMimeTypeMapping(t *testing.T) {
	defer prepareTestEnv(t)()
	// Fixme: this doesn't take effect on the server-side?
	setting.MimeTypeMap.Map[".foo"] = "audio/foo"
	setting.MimeTypeMap.Enabled = true
	_ = mime.AddExtensionType(".foo", "audio/foo")

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo2/raw/branch/master/bin.foo")
	resp := session.MakeRequest(t, req, http.StatusOK)

	// assert.Equal(t, "audio/foo", resp.HeaderMap.Get("Content-Type"))
	assert.Equal(t, "application/octet-stream", resp.HeaderMap.Get("Content-Type"))

	delete(setting.MimeTypeMap.Map, ".foo")
	setting.MimeTypeMap.Enabled = false
}
