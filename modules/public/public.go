// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package public

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

func CustomAssets() *assetfs.Layer {
	return assetfs.Local("custom", setting.CustomPath, "public")
}

func AssetFS() *assetfs.LayeredFS {
	return assetfs.Layered(CustomAssets(), BuiltinAssets())
}

type fileHandler struct {
	gzipContent sync.Map
}

// FileHandlerFunc implements the static handler for serving files in "public" assets
func FileHandlerFunc() http.HandlerFunc {
	assetFS := AssetFS()
	fh := fileHandler{}
	return func(resp http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" && req.Method != "HEAD" {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		fh.handleRequest(resp, req, assetFS, req.URL.Path)
	}
}

// parseAcceptEncoding parse Accept-Encoding: deflate, gzip;q=1.0, *;q=0.5 as compress methods
func parseAcceptEncoding(val string) container.Set[string] {
	parts := strings.Split(val, ";")
	types := make(container.Set[string])
	for _, v := range strings.Split(parts[0], ",") {
		types.Add(strings.TrimSpace(v))
	}
	return types
}

// setWellKnownContentType will set the Content-Type if the file is a well-known type.
// See the comments of detectWellKnownMimeType
func setWellKnownContentType(w http.ResponseWriter, file string) {
	mimeType := detectWellKnownMimeType(path.Ext(file))
	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}
}

func (fh *fileHandler) handleRequest(w http.ResponseWriter, req *http.Request, fs http.FileSystem, file string) {
	// actually, fs (http.FileSystem) is designed to be a safe interface, relative paths won't bypass its parent directory, it's also fine to do a clean here
	f, err := fs.Open(util.PathJoinRelX(file))
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Error("[Static] Open %q failed: %v", file, err)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error("[Static] %q exists, but fails to open: %v", file, err)
		return
	}

	// need to serve index file? (no at the moment)
	if fi.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	fh.serveContent(w, req, fi, fi.ModTime(), f)
}

// serveContent serve http content
func (fh *fileHandler) serveContent(w http.ResponseWriter, req *http.Request, fi os.FileInfo, modtime time.Time, content io.ReadSeeker) {
	setWellKnownContentType(w, fi.Name())

	encodings := parseAcceptEncoding(req.Header.Get("Accept-Encoding"))
	fileName := fi.Name()
	compressible := strings.HasSuffix(fileName, ".txt") || strings.HasSuffix(fileName, ".js") || strings.HasSuffix(fileName, ".css") || strings.HasSuffix(fileName, ".svg")
	compressible = compressible && fi.Size() > 512
	if encodings.Contains("gzip") && compressible {
		var compressedBytes []byte
		if compressed, ok := fh.gzipContent.Load(fileName); !ok {
			buf := &bytes.Buffer{}
			c := gzip.NewWriter(buf)
			_, _ = io.Copy(c, content)
			_ = c.Close()
			compressedBytes = buf.Bytes()
			fh.gzipContent.Store(fileName, compressedBytes)
		} else {
			compressedBytes = compressed.([]byte)
		}
		rdGzip := bytes.NewReader(compressedBytes)
		// all gzipped static files (from bindata) are managed by Gitea, so we can make sure every file has the correct ext name
		// then we can get the correct Content-Type, we do not need to do http.DetectContentType on the decompressed data
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		w.Header().Set("Content-Encoding", "gzip")
		httpcache.ServeContentWithCacheControl(w, req, fi.Name(), modtime, rdGzip)
		return
	}

	httpcache.ServeContentWithCacheControl(w, req, fi.Name(), modtime, content)
	return
}
