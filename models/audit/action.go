// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

type Action string

const (
	UserImpersonation               Action = "user:impersonation"
	UserCreate                      Action = "user:create"
	UserUpdate                      Action = "user:update"
	UserDelete                      Action = "user:delete"
	UserAuthenticationFailTwoFactor Action = "user:authentication:fail:twofactor"
	UserAuthenticationSource        Action = "user:authentication:source"
	UserActive                      Action = "user:active"
	UserRestricted                  Action = "user:restricted"
	UserAdmin                       Action = "user:admin"
	UserName                        Action = "user:name"
	UserPassword                    Action = "user:password"
	UserPasswordReset               Action = "user:password:reset"
	UserVisibility                  Action = "user:visibility"
	UserEmailPrimaryChange          Action = "user:email:primary"
	UserEmailAdd                    Action = "user:email:add"
	UserEmailActivate               Action = "user:email:activate"
	UserEmailRemove                 Action = "user:email:remove"
	UserTwoFactorEnable             Action = "user:twofactor:enable"
	UserTwoFactorRegenerate         Action = "user:twofactor:regenerate"
	UserTwoFactorDisable            Action = "user:twofactor:disable"
	UserWebAuthAdd                  Action = "user:webauth:add"
	UserWebAuthRemove               Action = "user:webauth:remove"
	UserExternalLoginAdd            Action = "user:externallogin:add"
	UserExternalLoginRemove         Action = "user:externallogin:remove"
	UserOpenIDAdd                   Action = "user:openid:add"
	UserOpenIDRemove                Action = "user:openid:remove"
	UserAccessTokenAdd              Action = "user:accesstoken:add"
	UserAccessTokenRemove           Action = "user:accesstoken:remove"
	UserOAuth2ApplicationAdd        Action = "user:oauth2application:add"
	UserOAuth2ApplicationUpdate     Action = "user:oauth2application:update"
	UserOAuth2ApplicationSecret     Action = "user:oauth2application:secret"
	UserOAuth2ApplicationGrant      Action = "user:oauth2application:grant"
	UserOAuth2ApplicationRevoke     Action = "user:oauth2application:revoke"
	UserOAuth2ApplicationRemove     Action = "user:oauth2application:remove"
	UserKeySSHAdd                   Action = "user:key:ssh:add"
	UserKeySSHRemove                Action = "user:key:ssh:remove"
	UserKeyPrincipalAdd             Action = "user:key:principal:add"
	UserKeyPrincipalRemove          Action = "user:key:principal:remove"
	UserKeyGPGAdd                   Action = "user:key:gpg:add"
	UserKeyGPGRemove                Action = "user:key:gpg:remove"
	UserSecretAdd                   Action = "user:secret:add"
	UserSecretUpdate                Action = "user:secret:update"
	UserSecretRemove                Action = "user:secret:remove"
	UserWebhookAdd                  Action = "user:webhook:add"
	UserWebhookUpdate               Action = "user:webhook:update"
	UserWebhookRemove               Action = "user:webhook:remove"

	OrganizationCreate                  Action = "organization:create"
	OrganizationUpdate                  Action = "organization:update"
	OrganizationDelete                  Action = "organization:delete"
	OrganizationName                    Action = "organization:name"
	OrganizationVisibility              Action = "organization:visibility"
	OrganizationTeamAdd                 Action = "organization:team:add"
	OrganizationTeamUpdate              Action = "organization:team:update"
	OrganizationTeamRemove              Action = "organization:team:remove"
	OrganizationTeamPermission          Action = "organization:team:permission"
	OrganizationTeamMemberAdd           Action = "organization:team:member:add"
	OrganizationTeamMemberRemove        Action = "organization:team:member:remove"
	OrganizationOAuth2ApplicationAdd    Action = "organization:oauth2application:add"
	OrganizationOAuth2ApplicationUpdate Action = "organization:oauth2application:update"
	OrganizationOAuth2ApplicationSecret Action = "organization:oauth2application:secret"
	OrganizationOAuth2ApplicationRemove Action = "organization:oauth2application:remove"
	OrganizationSecretAdd               Action = "organization:secret:add"
	OrganizationSecretUpdate            Action = "organization:secret:update"
	OrganizationSecretRemove            Action = "organization:secret:remove"
	OrganizationWebhookAdd              Action = "organization:webhook:add"
	OrganizationWebhookUpdate           Action = "organization:webhook:update"
	OrganizationWebhookRemove           Action = "organization:webhook:remove"

	RepositoryCreate                 Action = "repository:create"
	RepositoryCreateFork             Action = "repository:create:fork"
	RepositoryUpdate                 Action = "repository:update"
	RepositoryArchive                Action = "repository:archive"
	RepositoryUnarchive              Action = "repository:unarchive"
	RepositoryDelete                 Action = "repository:delete"
	RepositoryName                   Action = "repository:name"
	RepositoryVisibility             Action = "repository:visibility"
	RepositoryConvertFork            Action = "repository:convert:fork"
	RepositoryConvertMirror          Action = "repository:convert:mirror"
	RepositoryMirrorPushAdd          Action = "repository:mirror:push:add"
	RepositoryMirrorPushRemove       Action = "repository:mirror:push:remove"
	RepositorySigningVerification    Action = "repository:signingverification"
	RepositoryTransferStart          Action = "repository:transfer:start"
	RepositoryTransferAccept         Action = "repository:transfer:accept"
	RepositoryTransferReject         Action = "repository:transfer:reject"
	RepositoryWikiDelete             Action = "repository:wiki:delete"
	RepositoryCollaboratorAdd        Action = "repository:collaborator:add"
	RepositoryCollaboratorAccess     Action = "repository:collaborator:access"
	RepositoryCollaboratorRemove     Action = "repository:collaborator:remove"
	RepositoryCollaboratorTeamAdd    Action = "repository:collaborator:team:add"
	RepositoryCollaboratorTeamRemove Action = "repository:collaborator:team:remove"
	RepositoryBranchDefault          Action = "repository:branch:default"
	RepositoryBranchProtectionAdd    Action = "repository:branch:protection:add"
	RepositoryBranchProtectionUpdate Action = "repository:branch:protection:update"
	RepositoryBranchProtectionRemove Action = "repository:branch:protection:remove"
	RepositoryTagProtectionAdd       Action = "repository:tag:protection:add"
	RepositoryTagProtectionUpdate    Action = "repository:tag:protection:update"
	RepositoryTagProtectionRemove    Action = "repository:tag:protection:remove"
	RepositoryWebhookAdd             Action = "repository:webhook:add"
	RepositoryWebhookUpdate          Action = "repository:webhook:update"
	RepositoryWebhookRemove          Action = "repository:webhook:remove"
	RepositoryDeployKeyAdd           Action = "repository:deploykey:add"
	RepositoryDeployKeyRemove        Action = "repository:deploykey:remove"
	RepositorySecretAdd              Action = "repository:secret:add"
	RepositorySecretUpdate           Action = "repository:secret:update"
	RepositorySecretRemove           Action = "repository:secret:remove"

	SystemWebhookAdd                 Action = "system:webhook:add"
	SystemWebhookUpdate              Action = "system:webhook:update"
	SystemWebhookRemove              Action = "system:webhook:remove"
	SystemAuthenticationSourceAdd    Action = "system:authenticationsource:add"
	SystemAuthenticationSourceUpdate Action = "system:authenticationsource:update"
	SystemAuthenticationSourceRemove Action = "system:authenticationsource:remove"
	SystemOAuth2ApplicationAdd       Action = "system:oauth2application:add"
	SystemOAuth2ApplicationUpdate    Action = "system:oauth2application:update"
	SystemOAuth2ApplicationSecret    Action = "system:oauth2application:secret"
	SystemOAuth2ApplicationRemove    Action = "system:oauth2application:remove"
)
