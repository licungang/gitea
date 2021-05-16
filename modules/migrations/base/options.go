// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "code.gitea.io/gitea/modules/structs"

// MigrateOptions defines the way a repository gets migrated
// this is for internal usage by migrations module and func who interact with it
type MigrateOptions struct {
	// required: true
	CloneAddr             string `json:"clone_addr" binding:"Required"`
	CloneAddrEncrypted    string `json:"clone_addr_encrypted"`
	AuthUsername          string `json:"auth_username"`
	AuthPassword          string `json:"auth_password"`
	AuthPasswordEncrypted string `json:"auth_password_encrypted"`
	AuthToken             string `json:"auth_token"`
	AuthTokenEncrypted    string `json:"auth_token_encrypted"`
	// required: true
	UID int `json:"uid" binding:"Required"`
	// required: true
	RepoName        string `json:"repo_name" binding:"Required"`
	Mirror          bool   `json:"mirror"`
	LFS             bool   `json:"lfs"`
	LFSEndpoint     string `json:"lfs_endpoint"`
	Private         bool   `json:"private"`
	Description     string `json:"description"`
	OriginalURL     string
	GitServiceType  structs.GitServiceType
	Wiki            bool
	Issues          bool
	Milestones      bool
	Labels          bool
	Releases        bool
	Comments        bool
	PullRequests    bool
	ReleaseAssets   bool
	MigrateToRepoID int64
	MirrorInterval  string `json:"mirror_interval"`
}
