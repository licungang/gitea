// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package pulls

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// PRReview stores for a user - PR - commit combination which files the user has already viewed
type PRReview struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"NOT NULL UNIQUE(pull_commit_user)"`
	ViewedFiles map[string]bool    `xorm:"TEXT JSON"`                         // Stores for each of the changed files of a PR whether they have been viewed or not
	CommitSHA   string             `xorm:"NOT NULL UNIQUE(pull_commit_user)"` // Which commit was the head commit for the review?
	PullID      int64              `xorm:"NOT NULL UNIQUE(pull_commit_user)"` // Which PR was the review on?
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`                           // Is an accurate indicator of the order of commits as we do not expect it to be possible to make reviews on previous commits
}

func init() {
	db.RegisterModel(new(PRReview))
}

// GetReview returns the PRReview with all given values prefilled, whether or not it exists in the database.
// If the review didn't exist before in the database, it won't afterwards either.
// The returned boolean shows whether the review exists in the database
func GetReview(userID, pullID int64, commitSHA string) (*PRReview, bool, error) {
	review := &PRReview{UserID: userID, CommitSHA: commitSHA, PullID: pullID}
	has, err := db.GetEngine(db.DefaultContext).Get(review)
	return review, has, err
}

// UpdateReview updates the given review inside the database, regardless of whether it existed before or not
// The given map of viewed files will be merged with the previous review, if present
func UpdateReview(userID, pullID int64, commitSHA string, viewedFiles map[string]bool) error {
	review, exists, err := GetReview(userID, pullID, commitSHA)
	if err != nil {
		return err
	}

	if exists {
		review.ViewedFiles = mergeFiles(review.ViewedFiles, viewedFiles)
	} else if previousReview, err := getNewestReviewApartFrom(commitSHA, userID, pullID); err != nil {
		return err

		// Overwrite the viewed files of the previous review if present
	} else if previousReview != nil {
		review.ViewedFiles = mergeFiles(previousReview.ViewedFiles, viewedFiles)
	} else {
		review.ViewedFiles = viewedFiles
	}

	// Insert or Update review
	engine := db.GetEngine(db.DefaultContext)
	if !exists {
		_, err := engine.Insert(review)
		return err
	}
	_, err = engine.ID(review.ID).Update(review)
	return err
}

// mergeFiles merges the given maps of files with their viewing state into one map.
// Values from oldFiles will be overridden with values from newFiles
func mergeFiles(oldFiles, newFiles map[string]bool) map[string]bool {
	if oldFiles == nil {
		return newFiles
	} else if newFiles == nil {
		return oldFiles
	}

	for file, viewed := range newFiles {
		oldFiles[file] = viewed
	}
	return oldFiles
}

// GetNewestReview gets the newest review of the current user in the current PR.
// The returned PR Review will be nil if the user has not yet reviewed this PR.
func GetNewestReview(userID, pullID int64) (*PRReview, error) {
	var review PRReview
	has, err := db.GetEngine(db.DefaultContext).Where("user_id = ?", userID).And("pull_id = ?", pullID).OrderBy("updated_unix DESC").Limit(1).Get(&review)
	if err != nil || !has {
		return nil, err
	}
	return &review, err
}

// getNewestReviewApartFrom is like GetNewestReview, except that the second newest review will be returned if the newest review points at the given commit.
// The returned PR Review will be nil if the user has not yet reviewed this PR.
func getNewestReviewApartFrom(commitSHA string, userID, pullID int64) (*PRReview, error) {
	var review PRReview
	has, err := db.GetEngine(db.DefaultContext).Where("user_id = ?", userID).And("pull_id = ?", pullID).And("commit_sha != ?", commitSHA).OrderBy("updated_unix DESC").Limit(1).Get(&review)
	if err != nil || !has {
		return nil, err
	}
	return &review, err
}
