// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/core"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	bots_module "code.gitea.io/gitea/modules/bots"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/nektos/act/pkg/jobparser"
)

type botsNotifier struct {
	base.NullNotifier
}

var _ base.Notifier = &botsNotifier{}

// NewNotifier create a new botsNotifier notifier
func NewNotifier() base.Notifier {
	return &botsNotifier{}
}

func notifyIssue(issue *models.Issue, doer *user_model.User, evt webhook.HookEventType, payload string) {
	err := issue.LoadRepo(db.DefaultContext)
	if err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if issue.Repo.IsEmpty || issue.Repo.IsArchived {
		return
	}

	ref := issue.Ref
	if ref == "" {
		ref = issue.Repo.DefaultBranch
	}
	notify(issue.Repo, doer, payload, ref, evt)
}

func notify(repo *repo_model.Repository, doer *user_model.User, payload, ref string, evt webhook.HookEventType) {
	gitRepo, err := git.OpenRepository(context.Background(), repo.RepoPath())
	if err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	defer gitRepo.Close()

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		log.Error("gitRepo.GetCommit: %v", err)
		return
	}

	workflows, err := bots_module.DetectWorkflows(commit, evt)
	if err != nil {
		log.Error("DetectWorkflows: %v", err)
		return
	}

	if len(workflows) == 0 {
		log.Trace("repo %s with commit %s couldn't find workflows", repo.RepoPath(), commit.ID)
		return
	}

	for id, content := range workflows {
		run := bots_model.Run{
			Name:          commit.Message(),
			RepoID:        repo.ID,
			WorkflowID:    id,
			TriggerUserID: doer.ID,
			Ref:           ref,
			CommitSHA:     commit.ID.String(),
			Event:         evt,
			EventPayload:  payload,
			Status:        core.StatusPending,
		}
		jobs, err := jobparser.Parse(content) // TODO: parse with options
		if err != nil {
			log.Error("jobparser.Parse: %v", err)
			continue
		}
		if err := bots_model.InsertRun(&run, jobs); err != nil {
			log.Error("InsertRun: %v", err)
		}
	}
}

// TODO: implement all events
func (a *botsNotifier) NotifyNewIssue(issue *models.Issue, mentions []*user_model.User) {
	payload := map[string]interface{}{
		"issue": map[string]interface{}{
			"number": issue.Index,
		},
	}
	bs, err := json.Marshal(payload)
	if err != nil {
		log.Error("NotifyNewIssue: %v", err)
		return
	}
	notifyIssue(issue, issue.Poster, webhook.HookEventIssues, string(bs))
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func (a *botsNotifier) NotifyIssueChangeStatus(doer *user_model.User, issue *models.Issue, actionComment *models.Comment, closeOrReopen bool) {
}

func (a *botsNotifier) NotifyIssueChangeLabels(doer *user_model.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label,
) {
	payload := map[string]interface{}{
		"issue": map[string]interface{}{
			"number": issue.Index,
		},
	}
	bs, err := json.Marshal(payload)
	if err != nil {
		log.Error("NotifyNewIssue: %v", err)
		return
	}
	notifyIssue(issue, doer, webhook.HookEventIssueLabel, string(bs))
}

// NotifyCreateIssueComment notifies comment on an issue to notifiers
func (a *botsNotifier) NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
	issue *models.Issue, comment *models.Comment, mentions []*user_model.User) {
}

func (a *botsNotifier) NotifyNewPullRequest(pull *models.PullRequest, mentions []*user_model.User) {
}

func (a *botsNotifier) NotifyRenameRepository(doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
}

func (a *botsNotifier) NotifyTransferRepository(doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
}

func (a *botsNotifier) NotifyCreateRepository(doer *user_model.User, u *user_model.User, repo *repo_model.Repository) {
}

func (a *botsNotifier) NotifyForkRepository(doer *user_model.User, oldRepo, repo *repo_model.Repository) {
}

func (a *botsNotifier) NotifyPullRequestReview(pr *models.PullRequest, review *models.Review, comment *models.Comment, mentions []*user_model.User) {
}

func (*botsNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *user_model.User) {
}

func (a *botsNotifier) NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("webhook.NotifyPushCommits User: %s[%d] in %s[%d]", pusher.Name, pusher.ID, repo.FullName(), repo.ID))
	defer finished()

	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(ctx, repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	payload := &api.PushPayload{
		Ref:        opts.RefFullName,
		Before:     opts.OldCommitID,
		After:      opts.NewCommitID,
		CompareURL: setting.AppURL + commits.CompareURL,
		Commits:    apiCommits,
		HeadCommit: apiHeadCommit,
		Repo:       convert.ToRepo(repo, perm.AccessModeOwner),
		Pusher:     apiPusher,
		Sender:     apiPusher,
	}

	bs, err := json.Marshal(payload)
	if err != nil {
		log.Error("json.Marshal(payload) failed: %v", err)
		return
	}

	notify(repo, pusher, string(bs), opts.RefFullName, webhook.HookEventPush)
}

func (a *botsNotifier) NotifyCreateRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
}

func (a *botsNotifier) NotifyDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
}

func (a *botsNotifier) NotifySyncPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

func (a *botsNotifier) NotifySyncCreateRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
}

func (a *botsNotifier) NotifySyncDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
}

func (a *botsNotifier) NotifyNewRelease(rel *repo_model.Release) {
}
