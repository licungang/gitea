// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	//"code.gitea.io/gitea/models"
	//"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	wiki_service "code.gitea.io/gitea/services/wiki"

	"github.com/stretchr/testify/assert"
)

func TestAPIGetWikiPage(t *testing.T) {
	defer prepareTestEnv(t)()

	username := "user2"
	session := loginUser(t, username)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/Home", username, "repo1")

	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var page *api.WikiPage
	DecodeJSON(t, resp, &page)

	assert.Equal(t, &api.WikiPage{
		PageMeta: &api.PageMeta{
			Name:    "Home",
			SubURL:  "Home",
			Updated: "2017-11-26T20:31:18-08:00",
		},
		Title:       "",
		Content:     "# Home page\n\nThis is the home page!\n",
		CommitCount: 1,
		LastCommit: &api.WikiCommit{
			ID: "2c54faec6c45d31c1abfaecdab471eac6633738a",
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Ethan Koenig",
					Email: "ethantkoenig@gmail.com",
				},
				Date: "2017-11-26T20:31:18-08:00",
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Ethan Koenig",
					Email: "ethantkoenig@gmail.com",
				},
				Date: "2017-11-26T20:31:18-08:00",
			},
			Message: "Add Home.md\n",
		},
		Sidebar: "",
		Footer:  "",
	}, page)
}

func TestAPIGetWikiPages(t *testing.T) {
	defer prepareTestEnv(t)()

	username := "user2"
	session := loginUser(t, username)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/pages", username, "repo1")

	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var meta []*api.PageMeta
	DecodeJSON(t, resp, &meta)

	dummymeta := []*api.PageMeta{
		&api.PageMeta{
			Name:    "Home",
			SubURL:  "Home",
			Updated: "2017-11-26T20:31:18-08:00",
		},
		&api.PageMeta{
			Name:    "Page With Image",
			SubURL:  "Page-With-Image",
			Updated: "2019-01-24T20:41:55-05:00",
		},
		&api.PageMeta{
			Name:    "Page With Spaced Name",
			SubURL:  "Page-With-Spaced-Name",
			Updated: "2019-01-24T20:39:51-05:00",
		},
		&api.PageMeta{
			Name:    "Unescaped File",
			SubURL:  "Unescaped-File",
			Updated: "2021-07-19T18:42:46+02:00",
		},
	}

	assert.Equal(t, dummymeta, meta)
}

func TestAPINewWiki(t *testing.T) {
	for _, title := range []string{
		"New page",
		"&&&&",
	} {
		defer prepareTestEnv(t)()
		username := "user2"
		session := loginUser(t, username)
		token := getTokenForLoggedInUser(t, session)

		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/new?token=%s", username, "repo1", token)

		req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateWikiPageOptions{
			Title:   title,
			Content: "Wiki page content for API unit tests",
			Message: "",
		})
		session.MakeRequest(t, req, http.StatusNoContent)

		urlStrGet := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/%s", username, "repo1", wiki_service.NormalizeWikiName(title))
		reqGet := NewRequest(t, "GET", urlStrGet)
		session.MakeRequest(t, reqGet, http.StatusOK)
	}
}

func TestAPIEditWiki(t *testing.T) {
	defer prepareTestEnv(t)()
	username := "user2"
	session := loginUser(t, username)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/Page-With-Spaced-Name?token=%s", username, "repo1", token)

	req := NewRequestWithJSON(t, "PATCH", urlStr, &api.CreateWikiPageOptions{
		Title:   "edited title",
		Content: "Edited wiki page content for API unit tests",
		Message: "",
	})
	session.MakeRequest(t, req, http.StatusNoContent)
}

func TestAPIWikiRevisions(t *testing.T) {
	defer prepareTestEnv(t)()
	username := "user2"
	session := loginUser(t, username)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/revisions/Home", username, "repo1")

	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var revisions *api.WikiCommitList
	DecodeJSON(t, resp, &revisions)

	dummyrevisions := &api.WikiCommitList{
		WikiCommits: []*api.WikiCommit{
			{
				ID: "2c54faec6c45d31c1abfaecdab471eac6633738a",
				Author: &api.CommitUser{
					Identity: api.Identity{
						Name:  "Ethan Koenig",
						Email: "ethantkoenig@gmail.com",
					},
					Date: "2017-11-26T20:31:18-08:00",
				},
				Committer: &api.CommitUser{
					Identity: api.Identity{
						Name:  "Ethan Koenig",
						Email: "ethantkoenig@gmail.com",
					},
					Date: "2017-11-26T20:31:18-08:00",
				},
				Message: "Add Home.md\n",
			},
		},
		Count: 1,
	}

	assert.Equal(t, dummyrevisions, revisions)
}
