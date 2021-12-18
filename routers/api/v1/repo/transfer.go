// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	repo_service "code.gitea.io/gitea/services/repository"
)

// Transfer transfers the ownership of a repository
func Transfer(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/transfer repository repoTransfer
	// ---
	// summary: Transfer a repo ownership
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to transfer
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to transfer
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   description: "Transfer Options"
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/TransferRepoOption"
	// responses:
	//   "202":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := web.GetForm(ctx).(*api.TransferRepoOption)

	newOwner, err := user_model.GetUserByName(opts.NewOwner)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusNotFound, "", "The new owner does not exist or cannot be found")
			return
		}
		ctx.InternalServerError(err)
		return
	}

	if newOwner.Type == user_model.UserTypeOrganization {
		if !ctx.User.IsAdmin && newOwner.Visibility == api.VisibleTypePrivate && !models.OrgFromUser(newOwner).HasMemberWithUserID(ctx.User.ID) {
			// The user shouldn't know about this organization
			ctx.Error(http.StatusNotFound, "", "The new owner does not exist or cannot be found")
			return
		}
	}

	var teams []*models.Team
	if opts.TeamIDs != nil {
		if !newOwner.IsOrganization() {
			ctx.Error(http.StatusUnprocessableEntity, "repoTransfer", "Teams can only be added to organization-owned repositories")
			return
		}

		org := convert.ToOrganization(models.OrgFromUser(newOwner))
		for _, tID := range *opts.TeamIDs {
			team, err := models.GetTeamByID(tID)
			if err != nil {
				ctx.Error(http.StatusUnprocessableEntity, "team", fmt.Errorf("team %d not found", tID))
				return
			}

			if team.OrgID != org.ID {
				ctx.Error(http.StatusForbidden, "team", fmt.Errorf("team %d belongs not to org %d", tID, org.ID))
				return
			}

			teams = append(teams, team)
		}
	}

	if err := repo_service.StartRepositoryTransfer(ctx.User, newOwner, ctx.Repo.Repository, teams); err != nil {
		if models.IsErrRepoTransferInProgress(err) {
			ctx.Error(http.StatusConflict, "CreatePendingRepositoryTransfer", err)
			return
		}

		if repo_model.IsErrRepoAlreadyExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "CreatePendingRepositoryTransfer", err)
			return
		}

		ctx.InternalServerError(err)
		return
	}

	if ctx.Repo.Repository.Status == repo_model.RepositoryPendingTransfer {
		log.Trace("Repository transfer initiated: %s -> %s", ctx.Repo.Repository.FullName(), newOwner.Name)
		ctx.JSON(http.StatusCreated, convert.ToRepo(ctx.Repo.Repository, perm.AccessModeAdmin))
		return
	}

	log.Trace("Repository transferred: %s -> %s", ctx.Repo.Repository.FullName(), newOwner.Name)
	ctx.JSON(http.StatusAccepted, convert.ToRepo(ctx.Repo.Repository, perm.AccessModeAdmin))
}

// AcceptTransfer accept a repo transfer
func AcceptTransfer(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/transfer/accept repository acceptRepoTransfer
	// ---
	// summary: Accept a repo transfer
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to transfer
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to transfer
	//   type: string
	//   required: true
	// responses:
	//   "202":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := acceptOrRejectRepoTransfer(ctx, true)
	if ctx.Written() {
		return
	}
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "acceptOrRejectRepoTransfer", err)
		return
	}

	ctx.JSON(http.StatusAccepted, convert.ToRepo(ctx.Repo.Repository, ctx.Repo.AccessMode))

}

// RejectTransfer reject a repo transfer
func RejectTransfer(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/transfer/reject repository rejectRepoTransfer
	// ---
	// summary: Reject a repo transfer
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to transfer
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to transfer
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := acceptOrRejectRepoTransfer(ctx, false)
	if ctx.Written() {
		return
	}
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "acceptOrRejectRepoTransfer", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToRepo(ctx.Repo.Repository, ctx.Repo.AccessMode))
}

func acceptOrRejectRepoTransfer(ctx *context.APIContext, accept bool) error {
	repoTransfer, err := models.GetPendingRepositoryTransfer(ctx.Repo.Repository)
	if err != nil {
		return err
	}

	if err := repoTransfer.LoadAttributes(); err != nil {
		return err
	}

	if !repoTransfer.CanUserAcceptTransfer(ctx.User) {
		ctx.Error(http.StatusForbidden, "CanUserAcceptTransfer", nil)
		return fmt.Errorf("user does not have permissions to do this")
	}

	if accept {
		return repo_service.TransferOwnership(repoTransfer.Doer, repoTransfer.Recipient, ctx.Repo.Repository, repoTransfer.Teams)
	}

	return models.CancelRepositoryTransfer(ctx.Repo.Repository)
}
