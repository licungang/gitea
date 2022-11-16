// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT


package v1_14 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddChangedProtectedFilesPullRequestColumn(x *xorm.Engine) error {
	type PullRequest struct {
		ChangedProtectedFiles []string `xorm:"TEXT JSON"`
	}

	if err := x.Sync2(new(PullRequest)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
