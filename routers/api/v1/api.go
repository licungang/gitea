// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package v1 Gitea API.
//
// This documentation describes the Gitea API.
//
//	Schemes: http, https
//	BasePath: /api/v1
//	Version: {{AppVer | JSEscape | Safe}}
//	License: MIT http://opensource.org/licenses/MIT
//
//	Consumes:
//	- application/json
//	- text/plain
//
//	Produces:
//	- application/json
//	- text/html
//
//	Security:
//	- BasicAuth :
//	- Token :
//	- AccessToken :
//	- AuthorizationHeaderToken :
//	- SudoParam :
//	- SudoHeader :
//	- TOTPHeader :
//
//	SecurityDefinitions:
//	BasicAuth:
//	     type: basic
//	Token:
//	     type: apiKey
//	     name: token
//	     in: query
//	AccessToken:
//	     type: apiKey
//	     name: access_token
//	     in: query
//	AuthorizationHeaderToken:
//	     type: apiKey
//	     name: Authorization
//	     in: header
//	     description: API tokens must be prepended with "token" followed by a space.
//	SudoParam:
//	     type: apiKey
//	     name: sudo
//	     in: query
//	     description: Sudo API request as the user provided as the key. Admin privileges are required.
//	SudoHeader:
//	     type: apiKey
//	     name: Sudo
//	     in: header
//	     description: Sudo API request as the user provided as the key. Admin privileges are required.
//	TOTPHeader:
//	     type: apiKey
//	     name: X-GITEA-OTP
//	     in: header
//	     description: Must be used in combination with BasicAuth if two-factor authentication is enabled.
//
// swagger:meta
package v1

import (
	gocontext "context"
	"fmt"
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/activitypub"
	"code.gitea.io/gitea/routers/api/v1/admin"
	"code.gitea.io/gitea/routers/api/v1/misc"
	"code.gitea.io/gitea/routers/api/v1/notify"
	"code.gitea.io/gitea/routers/api/v1/org"
	"code.gitea.io/gitea/routers/api/v1/packages"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"code.gitea.io/gitea/routers/api/v1/settings"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/services/auth"
	context_service "code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"

	_ "code.gitea.io/gitea/routers/api/v1/swagger" // for swagger generation

	"gitea.com/go-chi/binding"
	"github.com/go-chi/cors"
)

func sudo() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		sudo := ctx.FormString("sudo")
		if len(sudo) == 0 {
			sudo = ctx.Req.Header.Get("Sudo")
		}

		if len(sudo) > 0 {
			if ctx.IsSigned && ctx.Doer.IsAdmin {
				user, err := user_model.GetUserByName(ctx, sudo)
				if err != nil {
					if user_model.IsErrUserNotExist(err) {
						ctx.NotFound()
					} else {
						ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
					}
					return
				}
				log.Trace("Sudo from (%s) to: %s", ctx.Doer.Name, user.Name)
				ctx.Doer = user
			} else {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "Only administrators allowed to sudo.",
				})
				return
			}
		}
	}
}

func repoAssignment() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		userName := ctx.Params("username")
		repoName := ctx.Params("reponame")

		var (
			owner *user_model.User
			err   error
		)

		// Check if the user is the same as the repository owner.
		if ctx.IsSigned && ctx.Doer.LowerName == strings.ToLower(userName) {
			owner = ctx.Doer
		} else {
			owner, err = user_model.GetUserByName(ctx, userName)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					if redirectUserID, err := user_model.LookupUserRedirect(userName); err == nil {
						context.RedirectToUser(ctx.Base, userName, redirectUserID)
					} else if user_model.IsErrUserRedirectNotExist(err) {
						ctx.NotFound("GetUserByName", err)
					} else {
						ctx.Error(http.StatusInternalServerError, "LookupUserRedirect", err)
					}
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner
		ctx.ContextUser = owner

		// Get repository.
		repo, err := repo_model.GetRepositoryByName(owner.ID, repoName)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				redirectRepoID, err := repo_model.LookupRedirect(owner.ID, repoName)
				if err == nil {
					context.RedirectToRepo(ctx.Base, redirectRepoID)
				} else if repo_model.IsErrRedirectNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "LookupRepoRedirect", err)
				}
			} else {
				ctx.Error(http.StatusInternalServerError, "GetRepositoryByName", err)
			}
			return
		}

		repo.Owner = owner
		ctx.Repo.Repository = repo

		if ctx.Doer != nil && ctx.Doer.ID == user_model.ActionsUserID {
			taskID := ctx.Data["ActionsTaskID"].(int64)
			task, err := actions_model.GetTaskByID(ctx, taskID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "actions_model.GetTaskByID", err)
				return
			}
			if task.RepoID != repo.ID {
				ctx.NotFound()
				return
			}

			if task.IsForkPullRequest {
				ctx.Repo.Permission.AccessMode = perm.AccessModeRead
			} else {
				ctx.Repo.Permission.AccessMode = perm.AccessModeWrite
			}

			if err := ctx.Repo.Repository.LoadUnits(ctx); err != nil {
				ctx.Error(http.StatusInternalServerError, "LoadUnits", err)
				return
			}
			ctx.Repo.Permission.Units = ctx.Repo.Repository.Units
			ctx.Repo.Permission.UnitsMode = make(map[unit.Type]perm.AccessMode)
			for _, u := range ctx.Repo.Repository.Units {
				ctx.Repo.Permission.UnitsMode[u.Type] = ctx.Repo.Permission.AccessMode
			}
		} else {
			ctx.Repo.Permission, err = access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
				return
			}
		}

		if !ctx.Repo.HasAccess() {
			ctx.NotFound()
			return
		}
	}
}

func reqPackageAccess(accessMode perm.AccessMode) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqPackageAccess", "user should have specific permission or be a site admin")
			return
		}
	}
}

// Contexter middleware already checks token for user sign in process.
func reqToken(requiredScopeCategory auth_model.AccessTokenScopeCategory) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		// If actions token is present
		if true == ctx.Data["IsActionsToken"] {
			return
		}

		// If OAuth2 token is present
		if _, ok := ctx.Data["ApiTokenScope"]; ctx.Data["IsApiToken"] == true && ok {
			// no scope required
			if requiredScopeCategory == auth_model.NoCategory {
				return
			}

			// get the scope from the request
			scope := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)

			// use the http method to determine the access level
			requiredScopeLevel := auth_model.Read
			if ctx.Req.Method == "POST" || ctx.Req.Method == "PUT" || ctx.Req.Method == "PATCH" {
				requiredScopeLevel = auth_model.Write
			} else if ctx.Req.Method == "DELETE" {
				requiredScopeLevel = auth_model.Delete
			}

			// get the required scope for the given access level and category
			requiredScope := auth_model.GetRequiredScope(requiredScopeLevel, requiredScopeCategory)

			// check if scope only applies to public resources
			publicOnly, err := scope.PublicOnly()
			if err != nil {
				ctx.Error(http.StatusForbidden, "reqToken", "parsing public resource scope failed: "+err.Error())
				return
			}

			if publicOnly && (requiredScopeCategory == auth_model.AccessTokenScopeCategoryRepository) &&
				ctx.Repo.Repository != nil && ctx.Repo.Repository.IsPrivate {
				ctx.Error(http.StatusForbidden, "reqToken", "token scope is limited to public repos: "+requiredScope)
				return
			}

			if publicOnly && (requiredScopeCategory == auth_model.AccessTokenScopeCategoryOrganization) &&
				ctx.Org.Organization != nil && ctx.Org.Organization.Visibility != structs.VisibleTypePublic {
				ctx.Error(http.StatusForbidden, "reqToken", "token scope is limited to public orgs: "+requiredScope)
				return
			}

			allow, err := scope.HasScope(requiredScope)
			if err != nil {
				ctx.Error(http.StatusForbidden, "reqToken", "checking scope failed: "+err.Error())
				return
			}

			if allow {
				return
			}

			ctx.Error(http.StatusForbidden, "reqToken", "token does not have required scope: "+requiredScope)
			return
		}
		if ctx.IsBasicAuth {
			ctx.CheckForOTP()
			return
		}
		if ctx.IsSigned {
			return
		}
		ctx.Error(http.StatusUnauthorized, "reqToken", "token is required")
	}
}

func reqExploreSignIn() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if setting.Service.Explore.RequireSigninView && !ctx.IsSigned {
			ctx.Error(http.StatusUnauthorized, "reqExploreSignIn", "you must be signed in to search for users")
		}
	}
}

func reqBasicAuth() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsBasicAuth {
			ctx.Error(http.StatusUnauthorized, "reqBasicAuth", "auth required")
			return
		}
		ctx.CheckForOTP()
	}
}

// reqSiteAdmin user should be the site admin
func reqSiteAdmin() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqSiteAdmin", "user should be the site admin")
			return
		}
	}
}

// reqOwner user should be the owner of the repo or site admin.
func reqOwner() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Repo.IsOwner() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqOwner", "user should be the owner of the repo")
			return
		}
	}
}

// reqAdmin user should be an owner or a collaborator with admin write of a repository, or site admin
func reqAdmin() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqAdmin", "user should be an owner or a collaborator with admin write of a repository")
			return
		}
	}
}

// reqRepoWriter user should have a permission to write to a repo, or be a site admin
func reqRepoWriter(unitTypes ...unit.Type) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.IsUserRepoWriter(unitTypes) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqRepoWriter", "user should have a permission to write to a repo")
			return
		}
	}
}

// reqRepoBranchWriter user should have a permission to write to a branch, or be a site admin
func reqRepoBranchWriter(ctx *context.APIContext) {
	options, ok := web.GetForm(ctx).(api.FileOptionInterface)
	if !ok || (!ctx.Repo.CanWriteToBranch(ctx.Doer, options.Branch()) && !ctx.IsUserSiteAdmin()) {
		ctx.Error(http.StatusForbidden, "reqRepoBranchWriter", "user should have a permission to write to this branch")
		return
	}
}

// reqRepoReader user should have specific read permission or be a repo admin or a site admin
func reqRepoReader(unitType unit.Type) func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Repo.CanRead(unitType) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqRepoReader", "user should have specific read permission or be a repo admin or a site admin")
			return
		}
	}
}

// reqAnyRepoReader user should have any permission to read repository or permissions of site admin
func reqAnyRepoReader() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Repo.HasAccess() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden, "reqAnyRepoReader", "user should have any permission to read repository or permissions of site admin")
			return
		}
	}
}

// reqOrgOwnership user should be an organization owner, or a site admin
func reqOrgOwnership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.IsUserSiteAdmin() {
			return
		}

		var orgID int64
		if ctx.Org.Organization != nil {
			orgID = ctx.Org.Organization.ID
		} else if ctx.Org.Team != nil {
			orgID = ctx.Org.Team.OrgID
		} else {
			ctx.Error(http.StatusInternalServerError, "", "reqOrgOwnership: unprepared context")
			return
		}

		isOwner, err := organization.IsOrganizationOwner(ctx, orgID, ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationOwner", err)
			return
		} else if !isOwner {
			if ctx.Org.Organization != nil {
				ctx.Error(http.StatusForbidden, "", "Must be an organization owner")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

// reqTeamMembership user should be an team member, or a site admin
func reqTeamMembership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.IsUserSiteAdmin() {
			return
		}
		if ctx.Org.Team == nil {
			ctx.Error(http.StatusInternalServerError, "", "reqTeamMembership: unprepared context")
			return
		}

		orgID := ctx.Org.Team.OrgID
		isOwner, err := organization.IsOrganizationOwner(ctx, orgID, ctx.Doer.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationOwner", err)
			return
		} else if isOwner {
			return
		}

		if isTeamMember, err := organization.IsTeamMember(ctx, orgID, ctx.Org.Team.ID, ctx.Doer.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "IsTeamMember", err)
			return
		} else if !isTeamMember {
			isOrgMember, err := organization.IsOrganizationMember(ctx, orgID, ctx.Doer.ID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "IsOrganizationMember", err)
			} else if isOrgMember {
				ctx.Error(http.StatusForbidden, "", "Must be a team member")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

// reqOrgMembership user should be an organization member, or a site admin
func reqOrgMembership() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.IsUserSiteAdmin() {
			return
		}

		var orgID int64
		if ctx.Org.Organization != nil {
			orgID = ctx.Org.Organization.ID
		} else if ctx.Org.Team != nil {
			orgID = ctx.Org.Team.OrgID
		} else {
			ctx.Error(http.StatusInternalServerError, "", "reqOrgMembership: unprepared context")
			return
		}

		if isMember, err := organization.IsOrganizationMember(ctx, orgID, ctx.Doer.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationMember", err)
			return
		} else if !isMember {
			if ctx.Org.Organization != nil {
				ctx.Error(http.StatusForbidden, "", "Must be an organization member")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

func reqGitHook() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if !ctx.Doer.CanEditGitHook() {
			ctx.Error(http.StatusForbidden, "", "must be allowed to edit Git hooks")
			return
		}
	}
}

// reqWebhooksEnabled requires webhooks to be enabled by admin.
func reqWebhooksEnabled() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if setting.DisableWebhooks {
			ctx.Error(http.StatusForbidden, "", "webhooks disabled by administrator")
			return
		}
	}
}

func orgAssignment(args ...bool) func(ctx *context.APIContext) {
	var (
		assignOrg  bool
		assignTeam bool
	)
	if len(args) > 0 {
		assignOrg = args[0]
	}
	if len(args) > 1 {
		assignTeam = args[1]
	}
	return func(ctx *context.APIContext) {
		ctx.Org = new(context.APIOrganization)

		var err error
		if assignOrg {
			ctx.Org.Organization, err = organization.GetOrgByName(ctx, ctx.Params(":org"))
			if err != nil {
				if organization.IsErrOrgNotExist(err) {
					redirectUserID, err := user_model.LookupUserRedirect(ctx.Params(":org"))
					if err == nil {
						context.RedirectToUser(ctx.Base, ctx.Params(":org"), redirectUserID)
					} else if user_model.IsErrUserRedirectNotExist(err) {
						ctx.NotFound("GetOrgByName", err)
					} else {
						ctx.Error(http.StatusInternalServerError, "LookupUserRedirect", err)
					}
				} else {
					ctx.Error(http.StatusInternalServerError, "GetOrgByName", err)
				}
				return
			}
			ctx.ContextUser = ctx.Org.Organization.AsUser()
		}

		if assignTeam {
			ctx.Org.Team, err = organization.GetTeamByID(ctx, ctx.ParamsInt64(":teamid"))
			if err != nil {
				if organization.IsErrTeamNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "GetTeamById", err)
				}
				return
			}
		}
	}
}

func mustEnableIssues(ctx *context.APIContext) {
	if !ctx.Repo.CanRead(unit.TypeIssues) {
		if log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					unit.TypeIssues,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					unit.TypeIssues,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.NotFound()
		return
	}
}

func mustAllowPulls(ctx *context.APIContext) {
	if !(ctx.Repo.Repository.CanEnablePulls() && ctx.Repo.CanRead(unit.TypePullRequests)) {
		if ctx.Repo.Repository.CanEnablePulls() && log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					unit.TypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					unit.TypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.NotFound()
		return
	}
}

func mustEnableIssuesOrPulls(ctx *context.APIContext) {
	if !ctx.Repo.CanRead(unit.TypeIssues) &&
		!(ctx.Repo.Repository.CanEnablePulls() && ctx.Repo.CanRead(unit.TypePullRequests)) {
		if ctx.Repo.Repository.CanEnablePulls() && log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v and %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					unit.TypeIssues,
					unit.TypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v and %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					unit.TypeIssues,
					unit.TypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.NotFound()
		return
	}
}

func mustEnableWiki(ctx *context.APIContext) {
	if !(ctx.Repo.CanRead(unit.TypeWiki)) {
		ctx.NotFound()
		return
	}
}

func mustNotBeArchived(ctx *context.APIContext) {
	if ctx.Repo.Repository.IsArchived {
		ctx.NotFound()
		return
	}
}

func mustEnableAttachments(ctx *context.APIContext) {
	if !setting.Attachment.Enabled {
		ctx.NotFound()
		return
	}
}

// bind binding an obj to a func(ctx *context.APIContext)
func bind[T any](_ T) any {
	return func(ctx *context.APIContext) {
		theObj := new(T) // create a new form obj for every request but not use obj directly
		errs := binding.Bind(ctx.Req, theObj)
		if len(errs) > 0 {
			ctx.Error(http.StatusUnprocessableEntity, "validationError", fmt.Sprintf("%s: %s", errs[0].FieldNames, errs[0].Error()))
			return
		}
		web.SetForm(ctx, theObj)
	}
}

// The OAuth2 plugin is expected to be executed first, as it must ignore the user id stored
// in the session (if there is a user id stored in session other plugins might return the user
// object for that id).
//
// The Session plugin is expected to be executed second, in order to skip authentication
// for users that have already signed in.
func buildAuthGroup() *auth.Group {
	group := auth.NewGroup(
		&auth.OAuth2{},
		&auth.HTTPSign{},
		&auth.Basic{}, // FIXME: this should be removed once we don't allow basic auth in API
	)
	specialAdd(group)

	return group
}

// Routes registers all v1 APIs routes to web application.
func Routes(ctx gocontext.Context) *web.Route {
	m := web.NewRoute()

	m.Use(securityHeaders())
	if setting.CORSConfig.Enabled {
		m.Use(cors.Handler(cors.Options{
			// Scheme:           setting.CORSConfig.Scheme, // FIXME: the cors middleware needs scheme option
			AllowedOrigins: setting.CORSConfig.AllowDomain,
			// setting.CORSConfig.AllowSubdomain // FIXME: the cors middleware needs allowSubdomain option
			AllowedMethods:   setting.CORSConfig.Methods,
			AllowCredentials: setting.CORSConfig.AllowCredentials,
			AllowedHeaders:   append([]string{"Authorization", "X-Gitea-OTP"}, setting.CORSConfig.Headers...),
			MaxAge:           int(setting.CORSConfig.MaxAge.Seconds()),
		}))
	}
	m.Use(context.APIContexter())

	group := buildAuthGroup()
	if err := group.Init(ctx); err != nil {
		log.Error("Could not initialize '%s' auth method, error: %s", group.Name(), err)
	}

	// Get user from session if logged in.
	m.Use(auth.APIAuth(group))

	m.Use(auth.VerifyAuthWithOptionsAPI(&auth.VerifyOptions{
		SignInRequired: setting.Service.RequireSignInView,
	}))

	m.Group("", func() {
		// Miscellaneous (no scope required)
		if setting.API.EnableSwagger {
			m.Get("/swagger", func(ctx *context.APIContext) {
				ctx.Redirect(setting.AppSubURL + "/api/swagger")
			})
		}
		m.Get("/version", misc.Version)
		if setting.Federation.Enabled {
			m.Get("/nodeinfo", misc.NodeInfo)
			m.Group("/activitypub", func() {
				// deprecated, remove in 1.20, use /user-id/{user-id} instead
				m.Group("/user/{username}", func() {
					m.Get("", activitypub.Person)
					m.Post("/inbox", activitypub.ReqHTTPSignature(), activitypub.PersonInbox)
				}, context_service.UserAssignmentAPI())
				m.Group("/user-id/{user-id}", func() {
					m.Get("", activitypub.Person)
					m.Post("/inbox", activitypub.ReqHTTPSignature(), activitypub.PersonInbox)
				}, context_service.UserIDAssignmentAPI())
			}, reqToken(auth_model.AccessTokenScopeCategoryActivityPub))
		}

		m.Group("", func() {
			m.Get("/signing-key.gpg", misc.SigningKey)
			m.Post("/markup", bind(api.MarkupOption{}), misc.Markup)
			m.Post("/markdown", bind(api.MarkdownOption{}), misc.Markdown)
			m.Post("/markdown/raw", misc.MarkdownRaw)
			m.Get("/gitignore/templates", misc.ListGitignoresTemplates)
			m.Get("/gitignore/templates/{name}", misc.GetGitignoreTemplateInfo)
			m.Get("/licenses", misc.ListLicenseTemplates)
			m.Get("/licenses/{name}", misc.GetLicenseTemplateInfo)
			m.Get("/label/templates", misc.ListLabelTemplates)
			m.Get("/label/templates/{name}", misc.GetLabelTemplate)
		}, reqToken(auth_model.AccessTokenScopeCategoryMisc))

		m.Group("/settings", func() {
			m.Get("/ui", settings.GetGeneralUISettings)
			m.Get("/api", settings.GetGeneralAPISettings)
			m.Get("/attachment", settings.GetGeneralAttachmentSettings)
			m.Get("/repository", settings.GetGeneralRepoSettings)
		}) // TODO scope? (Is this misc?)

		// Notifications (requires 'notification' scope)
		m.Group("/notifications", func() {
			m.Combo("").
				Get(notify.ListNotifications).
				Put(notify.ReadNotifications)
			m.Get("/new", notify.NewAvailable)
			m.Combo("/threads/{id}").
				Get(notify.GetThread).
				Patch(notify.ReadThread)
		}, reqToken(auth_model.AccessTokenScopeCategoryNotification))

		// Users (no scope required)
		m.Group("/users", func() {
			m.Get("/search", reqExploreSignIn(), user.Search)

			m.Group("/{username}", func() {
				m.Get("", reqExploreSignIn(), user.GetInfo)

				if setting.Service.EnableUserHeatmap {
					m.Get("/heatmap", user.GetUserHeatmapData)
				}

				m.Get("/repos", reqExploreSignIn(), user.ListUserRepos)
				m.Group("/tokens", func() {
					m.Combo("").Get(user.ListAccessTokens).
						Post(bind(api.CreateAccessTokenOption{}), user.CreateAccessToken)
					m.Combo("/{id}").Delete(user.DeleteAccessToken)
				}, reqBasicAuth())

				m.Get("/activities/feeds", user.ListUserActivityFeeds)
			}, context_service.UserAssignmentAPI())
		}, reqToken(auth_model.NoCategory)) // TODO add scope here?

		// (no scope required)
		m.Group("/users", func() {
			m.Group("/{username}", func() {
				m.Get("/keys", user.ListPublicKeys)
				m.Get("/gpg_keys", user.ListGPGKeys)

				m.Get("/followers", user.ListFollowers)
				m.Group("/following", func() {
					m.Get("", user.ListFollowing)
					m.Get("/{target}", user.CheckFollowing)
				})

				m.Get("/starred", user.GetStarredRepos)

				m.Get("/subscriptions", user.GetWatchedRepos)
			}, context_service.UserAssignmentAPI())
		}, reqToken(auth_model.NoCategory)) // TODO add scope here?

		m.Group("/user", func() {
			m.Get("", user.GetAuthenticatedUser)
			m.Group("/settings", func() {
				m.Get("", user.GetUserSettings)
				m.Patch("", bind(api.UserSettingsOptions{}), user.UpdateUserSettings)
			})
			m.Combo("/emails").Get(user.ListEmails).
				Post(bind(api.CreateEmailOption{}), user.AddEmail).
				Delete(bind(api.DeleteEmailOption{}), user.DeleteEmail)

			m.Get("/followers", user.ListMyFollowers)
			m.Group("/following", func() {
				m.Get("", user.ListMyFollowing)
				m.Group("/{username}", func() {
					m.Get("", user.CheckMyFollowing)
					m.Put("", user.Follow)
					m.Delete("", user.Unfollow)
				}, context_service.UserAssignmentAPI())
			})

			m.Group("/keys", func() {
				m.Combo("").Get(user.ListMyPublicKeys).
					Post(bind(api.CreateKeyOption{}), user.CreatePublicKey)
				m.Combo("/{id}").Get(user.GetPublicKey).
					Delete(user.DeletePublicKey)
			})

			m.Group("/applications", func() {
				m.Combo("/oauth2").
					Get(user.ListOauth2Applications).
					Post(bind(api.CreateOAuth2ApplicationOptions{}), user.CreateOauth2Application)
				m.Combo("/oauth2/{id}").
					Delete(user.DeleteOauth2Application).
					Patch(bind(api.CreateOAuth2ApplicationOptions{}), user.UpdateOauth2Application).
					Get(user.GetOauth2Application)
			})

			m.Group("/gpg_keys", func() {
				m.Combo("").Get(user.ListMyGPGKeys).
					Post(bind(api.CreateGPGKeyOption{}), user.CreateGPGKey)
				m.Combo("/{id}").Get(user.GetGPGKey).
					Delete(user.DeleteGPGKey)
			})
			m.Get("/gpg_key_token", user.GetVerificationToken)
			m.Post("/gpg_key_verify", bind(api.VerifyGPGKeyOption{}), user.VerifyUserGPGKey)

			m.Combo("/repos", reqToken(auth_model.AccessTokenScopeCategoryRepository)).Get(user.ListMyRepos).
				Post(bind(api.CreateRepoOption{}), repo.Create)

			m.Group("/starred", func() {
				m.Get("", user.GetMyStarredRepos)
				m.Group("/{username}/{reponame}", func() {
					m.Get("", user.IsStarring)
					m.Put("", user.Star)
					m.Delete("", user.Unstar)
				}, repoAssignment())
			}, reqToken(auth_model.AccessTokenScopeCategoryRepository))
			m.Get("/times", reqToken(auth_model.AccessTokenScopeCategoryRepository), repo.ListMyTrackedTimes)
			m.Get("/stopwatches", reqToken(auth_model.AccessTokenScopeCategoryRepository), repo.GetStopwatches)
			m.Get("/subscriptions", reqToken(auth_model.AccessTokenScopeCategoryRepository), user.GetMyWatchedRepos)
			m.Get("/teams", reqToken(auth_model.AccessTokenScopeCategoryRepository), org.ListUserTeams)
			m.Group("/hooks", func() {
				m.Combo("").Get(user.ListHooks).
					Post(bind(api.CreateHookOption{}), user.CreateHook)
				m.Combo("/{id}").Get(user.GetHook).
					Patch(bind(api.EditHookOption{}), user.EditHook).
					Delete(user.DeleteHook)
			}, reqWebhooksEnabled())
		}, reqToken(auth_model.AccessTokenScopeCategoryUser))

		// Repositories
		m.Post("/org/{org}/repos", reqToken(auth_model.AccessTokenScopeCategoryOrganization), bind(api.CreateRepoOption{}), repo.CreateOrgRepoDeprecated)

		m.Combo("/repositories/{id}", reqToken(auth_model.AccessTokenScopeCategoryRepository)).Get(repo.GetByID)

		m.Group("/repos", func() {
			m.Get("/search", repo.Search)

			m.Get("/issues/search", reqToken(auth_model.AccessTokenScopeCategoryIssue), repo.SearchIssues)

			m.Post("/migrate", reqToken(auth_model.AccessTokenScopeCategoryRepository), bind(api.MigrateRepoOptions{}), repo.Migrate)

			m.Group("/{username}/{reponame}", func() {
				m.Combo("").Get(reqAnyRepoReader(), repo.Get).
					Delete(reqOwner(), repo.Delete).
					Patch(reqAdmin(), bind(api.EditRepoOption{}), repo.Edit)
				m.Post("/generate", reqRepoReader(unit.TypeCode), bind(api.GenerateRepoOption{}), repo.Generate)
				m.Group("/transfer", func() {
					m.Post("", reqOwner(), bind(api.TransferRepoOption{}), repo.Transfer)
					m.Post("/accept", repo.AcceptTransfer)
					m.Post("/reject", repo.RejectTransfer)
				})
				m.Combo("/notifications", reqToken(auth_model.AccessTokenScopeCategoryNotification)).
					Get(notify.ListRepoNotifications).
					Put(notify.ReadRepoNotifications)
				m.Group("/hooks/git", func() {
					m.Combo("").Get(repo.ListGitHooks)
					m.Group("/{id}", func() {
						m.Combo("").Get(repo.GetGitHook).
							Patch(bind(api.EditGitHookOption{}), repo.EditGitHook).
							Delete(repo.DeleteGitHook)
					})
				}, reqAdmin(), reqGitHook(), context.ReferencesGitRepo(true))
				m.Group("/hooks", func() {
					m.Combo("").Get(repo.ListHooks).
						Post(bind(api.CreateHookOption{}), repo.CreateHook)
					m.Group("/{id}", func() {
						m.Combo("").Get(repo.GetHook).
							Patch(bind(api.EditHookOption{}), repo.EditHook).
							Delete(repo.DeleteHook)
						m.Post("/tests", context.ReferencesGitRepo(), context.RepoRefForAPI, repo.TestHook)
					})
				}, reqAdmin(), reqWebhooksEnabled())
				m.Group("/collaborators", func() {
					m.Get("", reqAnyRepoReader(), repo.ListCollaborators)
					m.Group("/{collaborator}", func() {
						m.Combo("").Get(reqAnyRepoReader(), repo.IsCollaborator).
							Put(reqAdmin(), bind(api.AddCollaboratorOption{}), repo.AddCollaborator).
							Delete(reqAdmin(), repo.DeleteCollaborator)
						m.Get("/permission", repo.GetRepoPermissions)
					})
				})
				m.Get("/assignees", reqAnyRepoReader(), repo.GetAssignees)
				m.Get("/reviewers", reqAnyRepoReader(), repo.GetReviewers)
				m.Group("/teams", func() {
					m.Get("", reqAnyRepoReader(), repo.ListTeams)
					m.Combo("/{team}").Get(reqAnyRepoReader(), repo.IsTeam).
						Put(reqAdmin(), repo.AddTeam).
						Delete(reqAdmin(), repo.DeleteTeam)
				})
				m.Get("/raw/*", context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetRawFile)
				m.Get("/media/*", context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetRawFileOrLFS)
				m.Get("/archive/*", reqRepoReader(unit.TypeCode), repo.GetArchive)
				m.Combo("/forks").Get(repo.ListForks).
					Post(reqRepoReader(unit.TypeCode), bind(api.CreateForkOption{}), repo.CreateFork)
				m.Group("/branches", func() {
					m.Get("", repo.ListBranches)
					m.Get("/*", repo.GetBranch)
					m.Delete("/*", reqRepoWriter(unit.TypeCode), repo.DeleteBranch)
					m.Post("", reqRepoWriter(unit.TypeCode), bind(api.CreateBranchRepoOption{}), repo.CreateBranch)
				}, context.ReferencesGitRepo(), reqRepoReader(unit.TypeCode))
				m.Group("/branch_protections", func() {
					m.Get("", repo.ListBranchProtections)
					m.Post("", bind(api.CreateBranchProtectionOption{}), repo.CreateBranchProtection)
					m.Group("/{name}", func() {
						m.Get("", repo.GetBranchProtection)
						m.Patch("", bind(api.EditBranchProtectionOption{}), repo.EditBranchProtection)
						m.Delete("", repo.DeleteBranchProtection)
					})
				}, reqAdmin())
				m.Group("/tags", func() {
					m.Get("", repo.ListTags)
					m.Get("/*", repo.GetTag)
					m.Post("", reqRepoWriter(unit.TypeCode), bind(api.CreateTagOption{}), repo.CreateTag)
					m.Delete("/*", repo.DeleteTag)
				}, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo(true))
				m.Group("/keys", func() {
					m.Combo("").Get(repo.ListDeployKeys).
						Post(bind(api.CreateKeyOption{}), repo.CreateDeployKey)
					m.Combo("/{id}").Get(repo.GetDeployKey).
						Delete(repo.DeleteDeploykey)
				}, reqAdmin())
				m.Group("/times", func() {
					m.Combo("").Get(repo.ListTrackedTimesByRepository)
					m.Combo("/{timetrackingusername}").Get(repo.ListTrackedTimesByUser)
				}, mustEnableIssues)
				m.Group("/wiki", func() {
					m.Combo("/page/{pageName}").
						Get(repo.GetWikiPage).
						Patch(mustNotBeArchived, reqRepoWriter(unit.TypeWiki), bind(api.CreateWikiPageOptions{}), repo.EditWikiPage).
						Delete(mustNotBeArchived, reqRepoWriter(unit.TypeWiki), repo.DeleteWikiPage)
					m.Get("/revisions/{pageName}", repo.ListPageRevisions)
					m.Post("/new", mustNotBeArchived, reqRepoWriter(unit.TypeWiki), bind(api.CreateWikiPageOptions{}), repo.NewWikiPage)
					m.Get("/pages", repo.ListWikiPages)
				}, mustEnableWiki)
				m.Group("/issues", func() {
					m.Combo("").Get(repo.ListIssues).
						Post(mustNotBeArchived, bind(api.CreateIssueOption{}), repo.CreateIssue)
					m.Get("/pinned", repo.ListPinnedIssues)
					m.Group("/comments", func() {
						m.Get("", repo.ListRepoIssueComments)
						m.Group("/{id}", func() {
							m.Combo("").
								Get(repo.GetIssueComment).
								Patch(mustNotBeArchived, bind(api.EditIssueCommentOption{}), repo.EditIssueComment).
								Delete(repo.DeleteIssueComment)
							m.Combo("/reactions").
								Get(repo.GetIssueCommentReactions).
								Post(bind(api.EditReactionOption{}), repo.PostIssueCommentReaction).
								Delete(bind(api.EditReactionOption{}), repo.DeleteIssueCommentReaction)
							m.Group("/assets", func() {
								m.Combo("").
									Get(repo.ListIssueCommentAttachments).
									Post(mustNotBeArchived, repo.CreateIssueCommentAttachment)
								m.Combo("/{asset}").
									Get(repo.GetIssueCommentAttachment).
									Patch(mustNotBeArchived, bind(api.EditAttachmentOptions{}), repo.EditIssueCommentAttachment).
									Delete(mustNotBeArchived, repo.DeleteIssueCommentAttachment)
							}, mustEnableAttachments)
						})
					})
					m.Group("/{index}", func() {
						m.Combo("").Get(repo.GetIssue).
							Patch(bind(api.EditIssueOption{}), repo.EditIssue).
							Delete(reqAdmin(), context.ReferencesGitRepo(), repo.DeleteIssue)
						m.Group("/comments", func() {
							m.Combo("").Get(repo.ListIssueComments).
								Post(mustNotBeArchived, bind(api.CreateIssueCommentOption{}), repo.CreateIssueComment)
							m.Combo("/{id}").Patch(bind(api.EditIssueCommentOption{}), repo.EditIssueCommentDeprecated).
								Delete(repo.DeleteIssueCommentDeprecated)
						})
						m.Get("/timeline", repo.ListIssueCommentsAndTimeline)
						m.Group("/labels", func() {
							m.Combo("").Get(repo.ListIssueLabels).
								Post(bind(api.IssueLabelsOption{}), repo.AddIssueLabels).
								Put(bind(api.IssueLabelsOption{}), repo.ReplaceIssueLabels).
								Delete(repo.ClearIssueLabels)
							m.Delete("/{id}", repo.DeleteIssueLabel)
						})
						m.Group("/times", func() {
							m.Combo("").
								Get(repo.ListTrackedTimes).
								Post(bind(api.AddTimeOption{}), repo.AddTime).
								Delete(repo.ResetIssueTime)
							m.Delete("/{id}", repo.DeleteTime)
						})
						m.Combo("/deadline").Post(bind(api.EditDeadlineOption{}), repo.UpdateIssueDeadline)
						m.Group("/stopwatch", func() {
							m.Post("/start", repo.StartIssueStopwatch)
							m.Post("/stop", repo.StopIssueStopwatch)
							m.Delete("/delete", repo.DeleteIssueStopwatch)
						})
						m.Group("/subscriptions", func() {
							m.Get("", repo.GetIssueSubscribers)
							m.Get("/check", repo.CheckIssueSubscription)
							m.Put("/{user}", repo.AddIssueSubscription)
							m.Delete("/{user}", repo.DelIssueSubscription)
						})
						m.Combo("/reactions").
							Get(repo.GetIssueReactions).
							Post(bind(api.EditReactionOption{}), repo.PostIssueReaction).
							Delete(bind(api.EditReactionOption{}), repo.DeleteIssueReaction)
						m.Group("/assets", func() {
							m.Combo("").
								Get(repo.ListIssueAttachments).
								Post(mustNotBeArchived, repo.CreateIssueAttachment)
							m.Combo("/{asset}").
								Get(repo.GetIssueAttachment).
								Patch(mustNotBeArchived, bind(api.EditAttachmentOptions{}), repo.EditIssueAttachment).
								Delete(mustNotBeArchived, repo.DeleteIssueAttachment)
						}, mustEnableAttachments)
						m.Combo("/dependencies").
							Get(repo.GetIssueDependencies).
							Post(mustNotBeArchived, bind(api.IssueMeta{}), repo.CreateIssueDependency).
							Delete(mustNotBeArchived, bind(api.IssueMeta{}), repo.RemoveIssueDependency)
						m.Combo("/blocks").
							Get(repo.GetIssueBlocks).
							Post(bind(api.IssueMeta{}), repo.CreateIssueBlocking).
							Delete(bind(api.IssueMeta{}), repo.RemoveIssueBlocking)
						m.Group("/pin", func() {
							m.Combo("").
								Post(reqAdmin(), repo.PinIssue).
								Delete(reqAdmin(), repo.UnpinIssue)
							m.Patch("/{position}", reqAdmin(), repo.MoveIssuePin)
						})
					})
				}, mustEnableIssuesOrPulls)
				m.Group("/labels", func() {
					m.Combo("").Get(repo.ListLabels).
						Post(reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.CreateLabelOption{}), repo.CreateLabel)
					m.Combo("/{id}").Get(repo.GetLabel).
						Patch(reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.EditLabelOption{}), repo.EditLabel).
						Delete(reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), repo.DeleteLabel)
				})
				m.Post("/markup", bind(api.MarkupOption{}), misc.Markup)
				m.Post("/markdown", bind(api.MarkdownOption{}), misc.Markdown)
				m.Post("/markdown/raw", misc.MarkdownRaw)
				m.Group("/milestones", func() {
					m.Combo("").Get(repo.ListMilestones).
						Post(reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.CreateMilestoneOption{}), repo.CreateMilestone)
					m.Combo("/{id}").Get(repo.GetMilestone).
						Patch(reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), bind(api.EditMilestoneOption{}), repo.EditMilestone).
						Delete(reqRepoWriter(unit.TypeIssues, unit.TypePullRequests), repo.DeleteMilestone)
				})
				m.Get("/stargazers", repo.ListStargazers)
				m.Get("/subscribers", repo.ListSubscribers)
				m.Group("/subscription", func() {
					m.Get("", user.IsWatching)
					m.Put("", user.Watch)
					m.Delete("", user.Unwatch)
				})
				m.Group("/releases", func() {
					m.Combo("").Get(repo.ListReleases).
						Post(reqRepoWriter(unit.TypeReleases), context.ReferencesGitRepo(), bind(api.CreateReleaseOption{}), repo.CreateRelease)
					m.Combo("/latest").Get(repo.GetLatestRelease)
					m.Group("/{id}", func() {
						m.Combo("").Get(repo.GetRelease).
							Patch(reqRepoWriter(unit.TypeReleases), context.ReferencesGitRepo(), bind(api.EditReleaseOption{}), repo.EditRelease).
							Delete(reqRepoWriter(unit.TypeReleases), repo.DeleteRelease)
						m.Group("/assets", func() {
							m.Combo("").Get(repo.ListReleaseAttachments).
								Post(reqRepoWriter(unit.TypeReleases), repo.CreateReleaseAttachment)
							m.Combo("/{asset}").Get(repo.GetReleaseAttachment).
								Patch(reqRepoWriter(unit.TypeReleases), bind(api.EditAttachmentOptions{}), repo.EditReleaseAttachment).
								Delete(reqRepoWriter(unit.TypeReleases), repo.DeleteReleaseAttachment)
						})
					})
					m.Group("/tags", func() {
						m.Combo("/{tag}").
							Get(repo.GetReleaseByTag).
							Delete(reqRepoWriter(unit.TypeReleases), repo.DeleteReleaseByTag)
					})
				}, reqRepoReader(unit.TypeReleases))
				m.Post("/mirror-sync", reqToken(auth_model.AccessTokenScopeCategoryRepository), reqRepoWriter(unit.TypeCode), repo.MirrorSync)
				m.Post("/push_mirrors-sync", reqAdmin(), reqToken(auth_model.AccessTokenScopeCategoryRepository), repo.PushMirrorSync)
				m.Group("/push_mirrors", func() {
					m.Combo("").Get(repo.ListPushMirrors).
						Post(bind(api.CreatePushMirrorOption{}), repo.AddPushMirror)
					m.Combo("/{name}").
						Delete(repo.DeletePushMirrorByRemoteName).
						Get(repo.GetPushMirrorByName)
				}, reqAdmin())

				m.Get("/editorconfig/{filename}", context.ReferencesGitRepo(), context.RepoRefForAPI, reqRepoReader(unit.TypeCode), repo.GetEditorconfig)
				m.Group("/pulls", func() {
					m.Combo("").Get(repo.ListPullRequests).
						Post(mustNotBeArchived, bind(api.CreatePullRequestOption{}), repo.CreatePullRequest)
					m.Get("/pinned", repo.ListPinnedPullRequests)
					m.Group("/{index}", func() {
						m.Combo("").Get(repo.GetPullRequest).
							Patch(bind(api.EditPullRequestOption{}), repo.EditPullRequest)
						m.Get(".{diffType:diff|patch}", repo.DownloadPullDiffOrPatch)
						m.Post("/update", repo.UpdatePullRequest)
						m.Get("/commits", repo.GetPullRequestCommits)
						m.Get("/files", repo.GetPullRequestFiles)
						m.Combo("/merge").Get(repo.IsPullRequestMerged).
							Post(mustNotBeArchived, bind(forms.MergePullRequestForm{}), repo.MergePullRequest).
							Delete(mustNotBeArchived, repo.CancelScheduledAutoMerge)
						m.Group("/reviews", func() {
							m.Combo("").
								Get(repo.ListPullReviews).
								Post(bind(api.CreatePullReviewOptions{}), repo.CreatePullReview)
							m.Group("/{id}", func() {
								m.Combo("").
									Get(repo.GetPullReview).
									Delete(repo.DeletePullReview).
									Post(bind(api.SubmitPullReviewOptions{}), repo.SubmitPullReview)
								m.Combo("/comments").
									Get(repo.GetPullReviewComments)
								m.Post("/dismissals", bind(api.DismissPullReviewOptions{}), repo.DismissPullReview)
								m.Post("/undismissals", repo.UnDismissPullReview)
							})
						})
						m.Combo("/requested_reviewers").
							Delete(bind(api.PullReviewRequestOptions{}), repo.DeleteReviewRequests).
							Post(bind(api.PullReviewRequestOptions{}), repo.CreateReviewRequests)
					})
				}, mustAllowPulls, reqRepoReader(unit.TypeCode), context.ReferencesGitRepo())
				m.Group("/statuses", func() {
					m.Combo("/{sha}").Get(repo.GetCommitStatuses).
						Post(reqRepoWriter(unit.TypeCode), bind(api.CreateStatusOption{}), repo.NewCommitStatus)
				}, reqRepoReader(unit.TypeCode))
				m.Group("/commits", func() {
					m.Get("", context.ReferencesGitRepo(), repo.GetAllCommits)
					m.Group("/{ref}", func() {
						m.Get("/status", repo.GetCombinedCommitStatusByRef)
						m.Get("/statuses", repo.GetCommitStatusesByRef)
					}, context.ReferencesGitRepo())
				}, reqRepoReader(unit.TypeCode))
				m.Group("/git", func() {
					m.Group("/commits", func() {
						m.Get("/{sha}", repo.GetSingleCommit)
						m.Get("/{sha}.{diffType:diff|patch}", repo.DownloadCommitDiffOrPatch)
					})
					m.Get("/refs", repo.GetGitAllRefs)
					m.Get("/refs/*", repo.GetGitRefs)
					m.Get("/trees/{sha}", repo.GetTree)
					m.Get("/blobs/{sha}", repo.GetBlob)
					m.Get("/tags/{sha}", repo.GetAnnotatedTag)
					m.Get("/notes/{sha}", repo.GetNote)
				}, context.ReferencesGitRepo(true), reqRepoReader(unit.TypeCode))
				m.Post("/diffpatch", reqRepoWriter(unit.TypeCode), bind(api.ApplyDiffPatchFileOptions{}), repo.ApplyDiffPatch)
				m.Group("/contents", func() {
					m.Get("", repo.GetContentsList)
					m.Post("", reqToken(auth_model.AccessTokenScopeRepo), bind(api.ChangeFilesOptions{}), reqRepoBranchWriter, repo.ChangeFiles)
					m.Get("/*", repo.GetContents)
					m.Group("/*", func() {
						m.Post("", bind(api.CreateFileOptions{}), reqRepoBranchWriter, repo.CreateFile)
						m.Put("", bind(api.UpdateFileOptions{}), reqRepoBranchWriter, repo.UpdateFile)
						m.Delete("", bind(api.DeleteFileOptions{}), reqRepoBranchWriter, repo.DeleteFile)
					})
				}, reqRepoReader(unit.TypeCode))
				m.Get("/signing-key.gpg", misc.SigningKey)
				m.Group("/topics", func() {
					m.Combo("").Get(repo.ListTopics).
						Put(reqAdmin(), bind(api.RepoTopicOptions{}), repo.UpdateTopics)
					m.Group("/{topic}", func() {
						m.Combo("").Put(repo.AddTopic).
							Delete(repo.DeleteTopic)
					}, reqAdmin())
				}, reqAnyRepoReader())
				m.Get("/issue_templates", context.ReferencesGitRepo(), repo.GetIssueTemplates)
				m.Get("/issue_config", context.ReferencesGitRepo(), repo.GetIssueConfig)
				m.Get("/issue_config/validate", context.ReferencesGitRepo(), repo.ValidateIssueConfig)
				m.Get("/languages", reqRepoReader(unit.TypeCode), repo.GetLanguages)
				m.Get("/activities/feeds", repo.ListRepoActivityFeeds)
				m.Get("/new_pin_allowed", repo.AreNewIssuePinsAllowed)
			}, repoAssignment())
		}, reqToken(auth_model.AccessTokenScopeCategoryRepository))

		// NOTE: these are Gitea package management API - see packages.CommonRoutes and packages.DockerContainerRoutes for endpoints that implement package manager APIs
		m.Group("/packages/{username}", func() {
			m.Group("/{type}/{name}/{version}", func() {
				m.Get("", packages.GetPackage)
				m.Delete("", reqPackageAccess(perm.AccessModeWrite), packages.DeletePackage)
				m.Get("/files", packages.ListPackageFiles)
			})
			m.Get("/", packages.ListPackages)
		}, context_service.UserAssignmentAPI(), context.PackageAssignmentAPI(), reqToken(auth_model.AccessTokenScopeCategoryPackage), reqPackageAccess(perm.AccessModeRead))

		// Organizations
		m.Get("/user/orgs", reqToken(auth_model.AccessTokenScopeCategoryOrganization), org.ListMyOrgs)
		m.Group("/users/{username}/orgs", func() {
			m.Get("", org.ListUserOrgs)
			m.Get("/{org}/permissions", org.GetUserOrgsPermissions)
		}, context_service.UserAssignmentAPI(), reqToken(auth_model.AccessTokenScopeCategoryOrganization)) // TODO does this require user or org scope?
		m.Post("/orgs", reqToken(auth_model.AccessTokenScopeCategoryOrganization), bind(api.CreateOrgOption{}), org.Create)
		m.Get("/orgs", reqToken(auth_model.AccessTokenScopeCategoryOrganization), org.GetAll)
		m.Group("/orgs/{org}", func() {
			m.Combo("").Get(org.Get).
				Patch(reqOrgOwnership(), bind(api.EditOrgOption{}), org.Edit).
				Delete(reqOrgOwnership(), org.Delete)
			m.Combo("/repos").Get(user.ListOrgRepos).
				Post(bind(api.CreateRepoOption{}), repo.CreateOrgRepo)
			m.Group("/members", func() {
				m.Get("", org.ListMembers)
				m.Combo("/{username}").Get(org.IsMember).
					Delete(reqOrgOwnership(), org.DeleteMember)
			})
			m.Group("/public_members", func() {
				m.Get("", org.ListPublicMembers)
				m.Combo("/{username}").Get(org.IsPublicMember).
					Put(reqOrgMembership(), org.PublicizeMember).
					Delete(reqOrgMembership(), org.ConcealMember)
			})
			m.Group("/teams", func() {
				m.Get("", org.ListTeams)
				m.Post("", reqOrgOwnership(), bind(api.CreateTeamOption{}), org.CreateTeam)
				m.Get("/search", org.SearchTeam)
			}, reqOrgMembership())
			m.Group("/labels", func() {
				m.Get("", org.ListLabels)
				m.Post("", reqOrgOwnership(), bind(api.CreateLabelOption{}), org.CreateLabel)
				m.Combo("/{id}").Get(org.GetLabel).
					Patch(reqOrgOwnership(), bind(api.EditLabelOption{}), org.EditLabel).
					Delete(reqOrgOwnership(), org.DeleteLabel)
			})
			m.Group("/hooks", func() {
				m.Combo("").Get(org.ListHooks).
					Post(bind(api.CreateHookOption{}), org.CreateHook)
				m.Combo("/{id}").Get(org.GetHook).
					Patch(bind(api.EditHookOption{}), org.EditHook).
					Delete(org.DeleteHook)
			}, reqOrgOwnership(), reqWebhooksEnabled())
			m.Get("/activities/feeds", org.ListOrgActivityFeeds)
		}, reqToken(auth_model.AccessTokenScopeCategoryOrganization), orgAssignment(true))
		m.Group("/teams/{teamid}", func() {
			m.Combo("").Get(org.GetTeam).
				Patch(reqOrgOwnership(), bind(api.EditTeamOption{}), org.EditTeam).
				Delete(reqOrgOwnership(), org.DeleteTeam)
			m.Group("/members", func() {
				m.Get("", org.GetTeamMembers)
				m.Combo("/{username}").
					Get(org.GetTeamMember).
					Put(reqOrgOwnership(), org.AddTeamMember).
					Delete(reqOrgOwnership(), org.RemoveTeamMember)
			})
			m.Group("/repos", func() {
				m.Get("", org.GetTeamRepos)
				m.Combo("/{org}/{reponame}").
					Put(org.AddTeamRepository). // TODO repo scope?
					Delete(org.RemoveTeamRepository).
					Get(org.GetTeamRepo)
			})
			m.Get("/activities/feeds", org.ListTeamActivityFeeds)
		}, orgAssignment(false, true), reqToken(auth_model.AccessTokenScopeCategoryOrganization), reqTeamMembership())

		m.Group("/admin", func() {
			m.Group("/cron", func() {
				m.Get("", admin.ListCronTasks)
				m.Post("/{task}", admin.PostCronTask)
			})
			m.Get("/orgs", admin.GetAllOrgs)
			m.Group("/users", func() {
				m.Get("", admin.SearchUsers)
				m.Post("", bind(api.CreateUserOption{}), admin.CreateUser)
				m.Group("/{username}", func() {
					m.Combo("").Patch(bind(api.EditUserOption{}), admin.EditUser).
						Delete(admin.DeleteUser)
					m.Group("/keys", func() {
						m.Post("", bind(api.CreateKeyOption{}), admin.CreatePublicKey)
						m.Delete("/{id}", admin.DeleteUserPublicKey)
					})
					m.Get("/orgs", org.ListUserOrgs)
					m.Post("/orgs", bind(api.CreateOrgOption{}), admin.CreateOrg)
					m.Post("/repos", bind(api.CreateRepoOption{}), admin.CreateRepo)
					m.Post("/rename", bind(api.RenameUserOption{}), admin.RenameUser)
				}, context_service.UserAssignmentAPI())
			})
			m.Group("/emails", func() {
				m.Get("", admin.GetAllEmails)
				m.Get("/search", admin.SearchEmail)
			})
			m.Group("/unadopted", func() {
				m.Get("", admin.ListUnadoptedRepositories)
				m.Post("/{username}/{reponame}", admin.AdoptRepository)
				m.Delete("/{username}/{reponame}", admin.DeleteUnadoptedRepository)
			})
			m.Group("/hooks", func() {
				m.Combo("").Get(admin.ListHooks).
					Post(bind(api.CreateHookOption{}), admin.CreateHook)
				m.Combo("/{id}").Get(admin.GetHook).
					Patch(bind(api.EditHookOption{}), admin.EditHook).
					Delete(admin.DeleteHook)
			})
		}, reqToken(auth_model.AccessTokenScopeCategoryAdmin), reqSiteAdmin())

		m.Group("/topics", func() {
			m.Get("/search", repo.TopicSearch)
		}) // TODO scope?
	}, sudo())

	return m
}

func securityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			// CORB: https://www.chromium.org/Home/chromium-security/corb-for-developers
			// http://stackoverflow.com/a/3146618/244009
			resp.Header().Set("x-content-type-options", "nosniff")
			next.ServeHTTP(resp, req)
		})
	}
}
