// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package url

import (
	"fmt"
	"net/url"
	stdurl "net/url"
	"regexp"
	"strings"
)

// URL represents a git remote URL
type URL struct {
	*stdurl.URL
	extraMark int // 0 no extra 1 scp 2 file path with no prefix
}

// String returns the URL's string
func (u *URL) String() string {
	switch u.extraMark {
	case 0:
		return u.String()
	case 1:
		return fmt.Sprintf("%s@%s:%s", u.User.Username(), u.Host, u.Path)
	case 2:
		return u.Path
	default:
		return ""
	}
}

var scpSyntaxRe = regexp.MustCompile(`^([a-zA-Z0-9-._~]+)@([a-zA-Z0-9._-]+):(.*)$`)

// Parse parse all kinds of git remote URL
func Parse(remote string) (*URL, error) {
	if strings.Contains(remote, "://") {
		u, err := stdurl.Parse(remote)
		if err != nil {
			return nil, err
		}
		return &URL{URL: u}, nil
	}

	if results := scpSyntaxRe.FindStringSubmatch(remote); results != nil {
		return &URL{
			URL: &stdurl.URL{
				Scheme: "ssh",
				User:   url.User(results[1]),
				Host:   results[2],
				Path:   results[3],
			},
			extraMark: 1,
		}, nil
	}

	return &URL{
		URL: &stdurl.URL{
			Scheme: "file",
			Path:   remote,
		},
		extraMark: 2,
	}, nil
}
