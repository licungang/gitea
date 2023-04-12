// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"io"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/automerge"
)

// CreateCommitStatus creates a new CommitStatus given a bunch of parameters
// NOTE: All text-values will be trimmed from whitespaces.
// Requires: Repo, Creator, SHA
func CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, creator *user_model.User, sha string, status *git_model.CommitStatus) error {
	repoPath := repo.RepoPath()

	// confirm that commit is exist
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return fmt.Errorf("OpenRepository[%s]: %w", repoPath, err)
	}
	defer closer.Close()

	if commit, err := gitRepo.GetCommit(sha); err != nil {
		gitRepo.Close()
		return fmt.Errorf("GetCommit[%s]: %w", sha, err)
	} else if len(sha) != git.SHAFullLength {
		// use complete commit sha
		sha = commit.ID.String()
	}
	gitRepo.Close()

	if err := git_model.NewCommitStatus(ctx, git_model.NewCommitStatusOptions{
		Repo:         repo,
		Creator:      creator,
		SHA:          sha,
		CommitStatus: status,
	}); err != nil {
		return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, sha, err)
	}

	if status.State.IsSuccess() {
		if err := automerge.MergeScheduledPullRequest(ctx, sha, repo); err != nil {
			return fmt.Errorf("MergeScheduledPullRequest[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, sha, err)
		}
	}

	return nil
}

// CountDivergingCommits determines how many commits a branch is ahead or behind the repository's base branch
func CountDivergingCommits(ctx context.Context, repo *repo_model.Repository, branch string) (*git.DivergeObject, error) {
	divergence, err := git.GetDivergingCommits(ctx, repo.RepoPath(), repo.DefaultBranch, branch)
	if err != nil {
		return nil, err
	}
	return &divergence, nil
}

// GetPayloadCommitVerification returns the verification information of a commit
func GetPayloadCommitVerification(ctx context.Context, commit *git.Commit) *structs.PayloadCommitVerification {
	verification := &structs.PayloadCommitVerification{}
	commitVerification := asymkey_model.ParseCommitWithSignature(ctx, commit)
	if commit.Signature != nil {
		verification.Signature = commit.Signature.Signature
		verification.Payload = commit.Signature.Payload
	}
	if commitVerification.SigningUser != nil {
		verification.Signer = &structs.PayloadUser{
			Name:  commitVerification.SigningUser.Name,
			Email: commitVerification.SigningUser.Email,
		}
	}
	verification.Verified = commitVerification.Verified
	verification.Reason = commitVerification.Reason
	if verification.Reason == "" && !verification.Verified {
		verification.Reason = "gpg.error.not_signed_commit"
	}
	return verification
}

// CreateCheckRun creates a new checkrun given a bunch of parameters
func CreateCheckRun(ctx context.Context, repo *repo_model.Repository, creator *user_model.User, opts *structs.CreateCheckRunOptions) (*git_model.CheckRun, error) {
	repoPath := repo.RepoPath()

	// confirm that commit is exist
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository[%s]: %w", repoPath, err)
	}
	defer closer.Close()

	if commit, err := gitRepo.GetCommit(opts.HeadSHA); err != nil {
		gitRepo.Close()
		return nil, fmt.Errorf("GetCommit[%s]: %w", opts.HeadSHA, err)
	} else if len(opts.HeadSHA) != git.SHAFullLength {
		// use complete commit sha
		opts.HeadSHA = commit.ID.String()
	}
	gitRepo.Close()

	opts2 := &git_model.NewCheckRunOptions{
		Repo:    repo,
		Creator: creator,
		HeadSHA: opts.HeadSHA,
		Name:    opts.Name,
		Status:  opts.Status,
		Output:  opts.Output,
	}
	if opts.Conclusion != nil {
		opts2.Conclusion = *opts.Conclusion
	}
	if opts.DetailsURL != nil {
		opts2.DetailsURL = *opts.DetailsURL
	}
	if opts.ExternalID != nil {
		opts2.ExternalID = *opts.ExternalID
	}
	if opts.StartedAt != nil {
		opts2.StartedAt = timeutil.TimeStamp(opts.StartedAt.Unix())
	}
	if opts.CompletedAt != nil {
		opts2.CompletedAt = timeutil.TimeStamp(opts.CompletedAt.Unix())
	}
	if opts2.Output != nil {
		err = loadPatchsForCheckRunOutput(gitRepo, opts2.HeadSHA, opts2.Output)
		if err != nil {
			return nil, err
		}
	}

	checkRrun, err := git_model.CreateCheckRun(ctx, opts2)
	if err != nil {
		return nil, err
	}

	if checkRrun.ToStatus(nil).State.IsSuccess() {
		if err := automerge.MergeScheduledPullRequest(ctx, opts.HeadSHA, repo); err != nil {
			return nil, fmt.Errorf("MergeScheduledPullRequest[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, creator.ID, opts.HeadSHA, err)
		}
	}

	return checkRrun, nil
}

func getPatch(gitRepo *git.Repository, treePath, commitID string, line int64) (string, error) {
	reader, writer := io.Pipe()
	defer func() {
		_ = reader.Close()
		_ = writer.Close()
	}()
	go func() {
		if err := git.GetRepoRawDiffForFile(gitRepo, commitID+"~1", commitID, git.RawDiffNormal, treePath, writer); err != nil {
			_ = writer.CloseWithError(fmt.Errorf("GetRawDiffForLine[%s, %s, %s, %s]: %w", gitRepo.Path, commitID+"~1", commitID, treePath, err))
			return
		}
		_ = writer.Close()
	}()

	return git.CutDiffAroundLine(reader, int64((&issues_model.Comment{Line: line}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines)
}

func loadPatchsForCheckRunOutput(gitRepo *git.Repository, sha string, checkRunOutput *structs.CheckRunOutput) error {
	//  path -> line : patch
	patchs := make(map[string]map[int64]string)

	for _, a := range checkRunOutput.Annotations {
		if a == nil || a.StartLine == nil || *a.StartLine == 0 || a.Path == nil || len(*a.Path) == 0 {
			continue
		}

		cacheExist := false
		if fileCache, ok := patchs[*a.Path]; ok {
			if lineCache, ok := fileCache[int64(*a.StartLine)]; ok {
				cacheExist = true
				a.Patch = lineCache
			}
		}

		if !cacheExist {
			patch, err := getPatch(gitRepo, *a.Path, sha, int64(*a.StartLine))
			if err != nil {
				return err
			}

			if _, ok := patchs[*a.Path]; !ok {
				patchs[*a.Path] = make(map[int64]string)
			}

			patchs[*a.Path][int64(*a.StartLine)] = patch
			a.Patch = patch
		}
	}

	return nil
}

func LoadPatchsForCheckRunOutput(ctx context.Context, repo *repo_model.Repository, sha string, checkRunOutput *structs.CheckRunOutput) error {
	repoPath := repo.RepoPath()

	// confirm that commit is exist
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return fmt.Errorf("OpenRepository[%s]: %w", repoPath, err)
	}
	defer closer.Close()

	return loadPatchsForCheckRunOutput(gitRepo, sha, checkRunOutput)
}
