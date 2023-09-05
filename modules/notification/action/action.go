// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package action

import (
	"context"
	"path"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"
)

type actionNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &actionNotifier{}

// NewNotifier create a new actionNotifier notifier
func NewNotifier() base.Notifier {
	return &actionNotifier{}
}

func (a *actionNotifier) NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	repo := issue.Repo

	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID:  issue.Poster.ID,
		ActUser:    issue.Poster,
		OpType:     activities_model.ActionCreateIssue,
		IssueIndex: issue.Index,
		RepoID:     repo.ID,
		Repo:       repo,
		IsPrivate:  repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func (a *actionNotifier) NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool) {
	// Compose comment action, could be plain comment, close or reopen issue/pull request.
	// This object will be used to notify watchers in the end of function.
	act := &activities_model.Action{
		ActUserID:  doer.ID,
		ActUser:    doer,
		IssueIndex: issue.Index,
		RepoID:     issue.Repo.ID,
		Repo:       issue.Repo,
		Comment:    actionComment,
		CommentID:  actionComment.ID,
		IsPrivate:  issue.Repo.IsPrivate,
	}
	// Check comment type.
	if closeOrReopen {
		act.OpType = activities_model.ActionCloseIssue
		if issue.IsPull {
			act.OpType = activities_model.ActionClosePullRequest
		}
	} else {
		act.OpType = activities_model.ActionReopenIssue
		if issue.IsPull {
			act.OpType = activities_model.ActionReopenPullRequest
		}
	}

	// Notify watchers for whatever action comes in, ignore if no action type.
	if err := activities_model.NotifyWatchers(ctx, act); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

// NotifyCreateIssueComment notifies comment on an issue to notifiers
func (a *actionNotifier) NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	act := &activities_model.Action{
		ActUserID:  doer.ID,
		ActUser:    doer,
		RepoID:     issue.Repo.ID,
		Repo:       issue.Repo,
		Comment:    comment,
		CommentID:  comment.ID,
		IssueIndex: issue.Index,
		IsPrivate:  issue.Repo.IsPrivate,
	}

	truncatedContent, truncatedRight := util.SplitStringAtByteN(comment.Content, 200)
	if truncatedRight != "" {
		// in case the content is in a Latin family language, we remove the last broken word.
		lastSpaceIdx := strings.LastIndex(truncatedContent, " ")
		if lastSpaceIdx != -1 && (len(truncatedContent)-lastSpaceIdx < 15) {
			truncatedContent = truncatedContent[:lastSpaceIdx] + "…"
		}
	}
	act.Content = truncatedContent

	if issue.IsPull {
		act.OpType = activities_model.ActionCommentPull
	} else {
		act.OpType = activities_model.ActionCommentIssue
	}

	// Notify watchers for whatever action comes in, ignore if no action type.
	if err := activities_model.NotifyWatchers(ctx, act); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyNewPullRequest(ctx context.Context, pull *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pull.LoadIssue(ctx); err != nil {
		log.Error("pull.LoadIssue: %v", err)
		return
	}
	if err := pull.Issue.LoadRepo(ctx); err != nil {
		log.Error("pull.Issue.LoadRepo: %v", err)
		return
	}
	if err := pull.Issue.LoadPoster(ctx); err != nil {
		log.Error("pull.Issue.LoadPoster: %v", err)
		return
	}

	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID:  pull.Issue.Poster.ID,
		ActUser:    pull.Issue.Poster,
		OpType:     activities_model.ActionCreatePullRequest,
		IssueIndex: pull.Issue.Index,
		RepoID:     pull.Issue.Repo.ID,
		Repo:       pull.Issue.Repo,
		IsPrivate:  pull.Issue.Repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyRenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    activities_model.ActionRenameRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyTransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    activities_model.ActionTransferRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   path.Join(oldOwnerName, repo.Name),
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    activities_model.ActionCreateRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("notify watchers '%d/%d': %v", doer.ID, repo.ID, err)
	}
}

func (a *actionNotifier) NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    activities_model.ActionCreateRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("notify watchers '%d/%d': %v", doer.ID, repo.ID, err)
	}
}

func (a *actionNotifier) NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := review.LoadReviewer(ctx); err != nil {
		log.Error("LoadReviewer '%d/%d': %v", review.ID, review.ReviewerID, err)
		return
	}
	if err := review.LoadCodeComments(ctx); err != nil {
		log.Error("LoadCodeComments '%d/%d': %v", review.Reviewer.ID, review.ID, err)
		return
	}

	actions := make([]*activities_model.Action, 0, 10)
	for _, lines := range review.CodeComments {
		for _, comments := range lines {
			for _, comm := range comments {
				actions = append(actions, &activities_model.Action{
					ActUserID:  review.Reviewer.ID,
					ActUser:    review.Reviewer,
					IssueIndex: review.Issue.Index,
					Content:    strings.Split(comm.Content, "\n")[0],
					OpType:     activities_model.ActionCommentPull,
					RepoID:     review.Issue.RepoID,
					Repo:       review.Issue.Repo,
					IsPrivate:  review.Issue.Repo.IsPrivate,
					Comment:    comm,
					CommentID:  comm.ID,
				})
			}
		}
	}

	if review.Type != issues_model.ReviewTypeComment || strings.TrimSpace(comment.Content) != "" {
		action := &activities_model.Action{
			ActUserID:  review.Reviewer.ID,
			ActUser:    review.Reviewer,
			IssueIndex: review.Issue.Index,
			Content:    strings.Split(comment.Content, "\n")[0],
			RepoID:     review.Issue.RepoID,
			Repo:       review.Issue.Repo,
			IsPrivate:  review.Issue.Repo.IsPrivate,
			Comment:    comment,
			CommentID:  comment.ID,
		}

		switch review.Type {
		case issues_model.ReviewTypeApprove:
			action.OpType = activities_model.ActionApprovePullRequest
		case issues_model.ReviewTypeReject:
			action.OpType = activities_model.ActionRejectPullRequest
		default:
			action.OpType = activities_model.ActionCommentPull
		}

		actions = append(actions, action)
	}

	if err := activities_model.NotifyWatchersActions(actions); err != nil {
		log.Error("notify watchers '%d/%d': %v", review.Reviewer.ID, review.Issue.RepoID, err)
	}
}

func (*actionNotifier) NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID:  doer.ID,
		ActUser:    doer,
		OpType:     activities_model.ActionMergePullRequest,
		IssueIndex: pr.Issue.Index,
		RepoID:     pr.Issue.Repo.ID,
		Repo:       pr.Issue.Repo,
		IsPrivate:  pr.Issue.Repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers [%d]: %v", pr.ID, err)
	}
}

func (*actionNotifier) NotifyAutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID:  doer.ID,
		ActUser:    doer,
		OpType:     activities_model.ActionAutoMergePullRequest,
		IssueIndex: pr.Issue.Index,
		RepoID:     pr.Issue.Repo.ID,
		Repo:       pr.Issue.Repo,
		IsPrivate:  pr.Issue.Repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers [%d]: %v", pr.ID, err)
	}
}

func (*actionNotifier) NotifyPullRevieweDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	reviewerName := review.Reviewer.Name
	if len(review.OriginalAuthor) > 0 {
		reviewerName = review.OriginalAuthor
	}
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID:  doer.ID,
		ActUser:    doer,
		OpType:     activities_model.ActionPullReviewDismissed,
		IssueIndex: review.Issue.Index,
		Content:    reviewerName,
		RepoID:     review.Issue.Repo.ID,
		Repo:       review.Issue.Repo,
		IsPrivate:  review.Issue.Repo.IsPrivate,
		CommentID:  comment.ID,
		Comment:    comment,
	}); err != nil {
		log.Error("NotifyWatchers [%d]: %v", review.Issue.ID, err)
	}
}

func (a *actionNotifier) NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	data, err := json.Marshal(commits)
	if err != nil {
		log.Error("Marshal: %v", err)
		return
	}

	opType := activities_model.ActionCommitRepo

	// Check it's tag push or branch.
	if opts.RefFullName.IsTag() {
		opType = activities_model.ActionPushTag
		if opts.IsDelRef() {
			opType = activities_model.ActionDeleteTag
		}
	} else if opts.IsDelRef() {
		opType = activities_model.ActionDeleteBranch
	}

	if err = activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: pusher.ID,
		ActUser:   pusher,
		OpType:    opType,
		Content:   string(data),
		RepoID:    repo.ID,
		Repo:      repo,
		RefName:   opts.RefFullName.String(),
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	opType := activities_model.ActionCommitRepo
	if refFullName.IsTag() {
		// has sent same action in `NotifyPushCommits`, so skip it.
		return
	}
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    opType,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName.String(),
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	opType := activities_model.ActionDeleteBranch
	if refFullName.IsTag() {
		// has sent same action in `NotifyPushCommits`, so skip it.
		return
	}
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    opType,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName.String(),
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifySyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	data, err := json.Marshal(commits)
	if err != nil {
		log.Error("json.Marshal: %v", err)
		return
	}

	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: repo.OwnerID,
		ActUser:   repo.MustOwner(ctx),
		OpType:    activities_model.ActionMirrorSyncPush,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   opts.RefFullName.String(),
		Content:   string(data),
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifySyncCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: repo.OwnerID,
		ActUser:   repo.MustOwner(ctx),
		OpType:    activities_model.ActionMirrorSyncCreate,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName.String(),
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifySyncDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: repo.OwnerID,
		ActUser:   repo.MustOwner(ctx),
		OpType:    activities_model.ActionMirrorSyncDelete,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName.String(),
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}
	if err := activities_model.NotifyWatchers(ctx, &activities_model.Action{
		ActUserID: rel.PublisherID,
		ActUser:   rel.Publisher,
		OpType:    activities_model.ActionPublishRelease,
		RepoID:    rel.RepoID,
		Repo:      rel.Repo,
		IsPrivate: rel.Repo.IsPrivate,
		Content:   rel.Title,
		RefName:   rel.TagName, // FIXME: use a full ref name?
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}
