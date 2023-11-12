// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	"net/http"
	"sort"

	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/audit"
	"code.gitea.io/gitea/services/auth/source/oauth2"
)

const (
	tplSettingsSecurity    base.TplName = "user/settings/security/security"
	tplSettingsTwofaEnroll base.TplName = "user/settings/security/twofa_enroll"
)

// Security render change user's password page and 2FA
func Security(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.security")
	ctx.Data["PageIsSettingsSecurity"] = true

	if ctx.FormString("openid.return_to") != "" {
		settingsOpenIDVerify(ctx)
		return
	}

	loadSecurityData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsSecurity)
}

// DeleteAccountLink delete a single account link
func DeleteAccountLink(ctx *context.Context) {
	elu := &user_model.ExternalLoginUser{UserID: ctx.Doer.ID, LoginSourceID: ctx.FormInt64("id")}
	if has, err := user_model.GetExternalLogin(ctx, elu); err != nil || !has {
		if !has {
			err = user_model.ErrExternalLoginUserNotExist{UserID: elu.UserID, LoginSourceID: elu.LoginSourceID}
		}
		ctx.ServerError("RemoveAccountLink", err)
		return
	}

	if _, err := user_model.RemoveAccountLink(ctx, ctx.Doer, elu.LoginSourceID); err != nil {
		ctx.Flash.Error("RemoveAccountLink: " + err.Error())
		return
	} else {
		audit.Record(audit.UserExternalLoginRemove, ctx.Doer, ctx.Doer, elu, "Removed external login %s for user %s.", elu.ExternalID, ctx.Doer.Name)

		ctx.Flash.Success(ctx.Tr("settings.remove_account_link_success"))
	}

	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/security")
}

func loadSecurityData(ctx *context.Context) {
	enrolled, err := auth_model.HasTwoFactorByUID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}
	ctx.Data["TOTPEnrolled"] = enrolled

	credentials, err := auth_model.GetWebAuthnCredentialsByUID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetWebAuthnCredentialsByUID", err)
		return
	}
	ctx.Data["WebAuthnCredentials"] = credentials

	tokens, err := auth_model.ListAccessTokens(ctx, auth_model.ListAccessTokensOptions{UserID: ctx.Doer.ID})
	if err != nil {
		ctx.ServerError("ListAccessTokens", err)
		return
	}
	ctx.Data["Tokens"] = tokens

	accountLinks, err := user_model.ListAccountLinks(ctx, ctx.Doer)
	if err != nil {
		ctx.ServerError("ListAccountLinks", err)
		return
	}

	// map the provider display name with the AuthSource
	sources := make(map[*auth_model.Source]string)
	for _, externalAccount := range accountLinks {
		if authSource, err := auth_model.GetSourceByID(ctx, externalAccount.LoginSourceID); err == nil {
			var providerDisplayName string

			type DisplayNamed interface {
				DisplayName() string
			}

			type Named interface {
				Name() string
			}

			if displayNamed, ok := authSource.Cfg.(DisplayNamed); ok {
				providerDisplayName = displayNamed.DisplayName()
			} else if named, ok := authSource.Cfg.(Named); ok {
				providerDisplayName = named.Name()
			} else {
				providerDisplayName = authSource.Name
			}
			sources[authSource] = providerDisplayName
		}
	}
	ctx.Data["AccountLinks"] = sources

	authSources, err := auth_model.FindSources(ctx, auth_model.FindSourcesOptions{
		IsActive:  util.OptionalBoolNone,
		LoginType: auth_model.OAuth2,
	})
	if err != nil {
		ctx.ServerError("FindSources", err)
		return
	}

	var orderedOAuth2Names []string
	oauth2Providers := make(map[string]oauth2.Provider)
	for _, source := range authSources {
		provider, err := oauth2.CreateProviderFromSource(source)
		if err != nil {
			ctx.ServerError("CreateProviderFromSource", err)
			return
		}
		oauth2Providers[source.Name] = provider
		if source.IsActive {
			orderedOAuth2Names = append(orderedOAuth2Names, source.Name)
		}
	}

	sort.Strings(orderedOAuth2Names)

	ctx.Data["OrderedOAuth2Names"] = orderedOAuth2Names
	ctx.Data["OAuth2Providers"] = oauth2Providers

	openid, err := user_model.GetUserOpenIDs(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = openid
}
