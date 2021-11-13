// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/git"
	user_model "code.gitea.io/gitea/models/user"
)

// ConvertFromGitCommit converts git commits into SignCommitWithStatuses
func ConvertFromGitCommit(commits []*git.Commit, repo *Repository) []*SignCommitWithStatuses {
	return ParseCommitsWithStatus(
		ParseCommitsWithSignature(
			user_model.ValidateCommitsWithEmails(commits),
			repo,
		),
		repo,
	)
}
