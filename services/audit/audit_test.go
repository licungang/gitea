// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	audit_model "code.gitea.io/gitea/models/audit"
	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	organization_model "code.gitea.io/gitea/models/organization"
	repository_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/web/middleware"

	"github.com/stretchr/testify/assert"
)

func TestBuildEvent(t *testing.T) {
	equal := func(expected, e *Event) {
		expected.Time = time.Time{}
		e.Time = time.Time{}

		assert.Equal(t, expected, e)
	}

	ctx := context.Background()

	u := &user_model.User{ID: 1, Name: "TestUser"}
	r := &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"}
	m := &repository_model.PushMirror{ID: 4}
	doer := &user_model.User{ID: 2, Name: "Doer"}

	equal(
		&Event{
			Action:  audit_model.UserUpdate,
			Doer:    TypeDescriptor{Type: "user", PrimaryKey: int64(2), FriendlyName: "Doer", Target: doer},
			Scope:   TypeDescriptor{Type: "user", PrimaryKey: int64(1), FriendlyName: "TestUser", Target: u},
			Target:  TypeDescriptor{Type: "user", PrimaryKey: int64(1), FriendlyName: "TestUser", Target: u},
			Message: "Updated settings of user TestUser.",
		},
		BuildEvent(
			ctx,
			audit_model.UserUpdate,
			doer,
			u,
			u,
			"Updated settings of user %s.",
			u.Name,
		),
	)
	equal(
		&Event{
			Action:  audit_model.RepositoryMirrorPushAdd,
			Doer:    TypeDescriptor{Type: "user", PrimaryKey: int64(2), FriendlyName: "Doer", Target: doer},
			Scope:   TypeDescriptor{Type: "repository", PrimaryKey: int64(3), FriendlyName: "TestUser/TestRepo", Target: r},
			Target:  TypeDescriptor{Type: "push_mirror", PrimaryKey: int64(4), FriendlyName: "", Target: m},
			Message: "Added push mirror for repository TestUser/TestRepo.",
		},
		BuildEvent(
			ctx,
			audit_model.RepositoryMirrorPushAdd,
			doer,
			r,
			m,
			"Added push mirror for repository %s.",
			r.FullName(),
		),
	)

	e := BuildEvent(ctx, audit_model.UserUpdate, doer, u, u, "")
	assert.Empty(t, e.IPAddress)

	ctx = middleware.WithContextRequest(ctx, &http.Request{RemoteAddr: "127.0.0.1:1234"})

	e = BuildEvent(ctx, audit_model.UserUpdate, doer, u, u, "")
	assert.Equal(t, "127.0.0.1", e.IPAddress)
}

func TestScopeToDescription(t *testing.T) {
	cases := []struct {
		ShouldPanic bool
		Scope       any
		Expected    TypeDescriptor
	}{
		{
			Scope:    nil,
			Expected: TypeDescriptor{Type: audit_model.TypeSystem, PrimaryKey: 0, FriendlyName: "System"},
		},
		{
			Scope:    &user_model.User{ID: 1, Name: "TestUser"},
			Expected: TypeDescriptor{Type: audit_model.TypeUser, PrimaryKey: int64(1), FriendlyName: "TestUser"},
		},
		{
			Scope:    &organization_model.Organization{ID: 2, Name: "TestOrg"},
			Expected: TypeDescriptor{Type: audit_model.TypeOrganization, PrimaryKey: int64(2), FriendlyName: "TestOrg"},
		},
		{
			Scope:    &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"},
			Expected: TypeDescriptor{Type: audit_model.TypeRepository, PrimaryKey: int64(3), FriendlyName: "TestUser/TestRepo"},
		},
		{
			ShouldPanic: true,
			Scope:       &organization_model.Team{ID: 345, Name: "Repo345"},
		},
		{
			ShouldPanic: true,
			Scope:       1234,
		},
	}
	for _, c := range cases {
		c.Expected.Target = c.Scope

		if c.ShouldPanic {
			assert.Panics(t, func() {
				_ = scopeToDescription(c.Scope)
			})
		} else {
			assert.Equal(t, c.Expected, scopeToDescription(c.Scope), "Unexpected descriptor for scope: %T", c.Scope)
		}
	}
}

func TestTypeToDescription(t *testing.T) {
	cases := []struct {
		ShouldPanic bool
		Type        any
		Expected    TypeDescriptor
	}{
		{
			ShouldPanic: true,
			Type:        nil,
		},
		{
			Type:     &user_model.User{ID: 1, Name: "TestUser"},
			Expected: TypeDescriptor{Type: audit_model.TypeUser, PrimaryKey: int64(1), FriendlyName: "TestUser"},
		},
		{
			Type:     &organization_model.Organization{ID: 2, Name: "TestOrg"},
			Expected: TypeDescriptor{Type: audit_model.TypeOrganization, PrimaryKey: int64(2), FriendlyName: "TestOrg"},
		},
		{
			Type:     &user_model.EmailAddress{ID: 3, Email: "user@gitea.com"},
			Expected: TypeDescriptor{Type: audit_model.TypeEmailAddress, PrimaryKey: int64(3), FriendlyName: "user@gitea.com"},
		},
		{
			Type:     &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"},
			Expected: TypeDescriptor{Type: audit_model.TypeRepository, PrimaryKey: int64(3), FriendlyName: "TestUser/TestRepo"},
		},
		{
			Type:     &organization_model.Team{ID: 4, Name: "TestTeam"},
			Expected: TypeDescriptor{Type: audit_model.TypeTeam, PrimaryKey: int64(4), FriendlyName: "TestTeam"},
		},
		{
			Type:     &auth_model.TwoFactor{ID: 5},
			Expected: TypeDescriptor{Type: audit_model.TypeTwoFactor, PrimaryKey: int64(5), FriendlyName: ""},
		},
		{
			Type:     &auth_model.WebAuthnCredential{ID: 6, Name: "TestCredential"},
			Expected: TypeDescriptor{Type: audit_model.TypeWebAuthnCredential, PrimaryKey: int64(6), FriendlyName: "TestCredential"},
		},
		{
			Type:     &user_model.UserOpenID{ID: 7, URI: "test://uri"},
			Expected: TypeDescriptor{Type: audit_model.TypeOpenID, PrimaryKey: int64(7), FriendlyName: "test://uri"},
		},
		{
			Type:     &auth_model.AccessToken{ID: 8, Name: "TestToken"},
			Expected: TypeDescriptor{Type: audit_model.TypeAccessToken, PrimaryKey: int64(8), FriendlyName: "TestToken"},
		},
		{
			Type:     &auth_model.OAuth2Application{ID: 9, Name: "TestOAuth2Application"},
			Expected: TypeDescriptor{Type: audit_model.TypeOAuth2Application, PrimaryKey: int64(9), FriendlyName: "TestOAuth2Application"},
		},
		{
			Type:     &auth_model.OAuth2Grant{ID: 10},
			Expected: TypeDescriptor{Type: audit_model.TypeOAuth2Grant, PrimaryKey: int64(10), FriendlyName: ""},
		},
		{
			Type:     &auth_model.Source{ID: 11, Name: "TestSource"},
			Expected: TypeDescriptor{Type: audit_model.TypeAuthenticationSource, PrimaryKey: int64(11), FriendlyName: "TestSource"},
		},
		/*{
			Type:     &user_model.ExternalLoginUser{ExternalID: "12"},
			Expected: TypeDescriptor{Type: audit_model.TypeExternalLoginUser, PrimaryKey: "12", FriendlyName: "12"},
		},*/
		{
			Type:     &asymkey_model.PublicKey{ID: 13, Fingerprint: "TestPublicKey"},
			Expected: TypeDescriptor{Type: audit_model.TypePublicKey, PrimaryKey: int64(13), FriendlyName: "TestPublicKey"},
		},
		{
			Type:     &asymkey_model.GPGKey{ID: 14, KeyID: "TestGPGKey"},
			Expected: TypeDescriptor{Type: audit_model.TypeGPGKey, PrimaryKey: int64(14), FriendlyName: "TestGPGKey"},
		},
		{
			Type:     &secret_model.Secret{ID: 15, Name: "TestSecret"},
			Expected: TypeDescriptor{Type: audit_model.TypeSecret, PrimaryKey: int64(15), FriendlyName: "TestSecret"},
		},
		{
			Type:     &webhook_model.Webhook{ID: 16, URL: "test://webhook"},
			Expected: TypeDescriptor{Type: audit_model.TypeWebhook, PrimaryKey: int64(16), FriendlyName: "test://webhook"},
		},
		{
			Type:     &git_model.ProtectedTag{ID: 17, NamePattern: "TestProtectedTag"},
			Expected: TypeDescriptor{Type: audit_model.TypeProtectedTag, PrimaryKey: int64(17), FriendlyName: "TestProtectedTag"},
		},
		{
			Type:     &git_model.ProtectedBranch{ID: 18, RuleName: "TestProtectedBranch"},
			Expected: TypeDescriptor{Type: audit_model.TypeProtectedBranch, PrimaryKey: int64(18), FriendlyName: "TestProtectedBranch"},
		},
		{
			Type:     &repository_model.PushMirror{ID: 19},
			Expected: TypeDescriptor{Type: audit_model.TypePushMirror, PrimaryKey: int64(19), FriendlyName: ""},
		},
		{
			Type:     &models.RepoTransfer{ID: 20},
			Expected: TypeDescriptor{Type: audit_model.TypeRepoTransfer, PrimaryKey: int64(20), FriendlyName: ""},
		},
		{
			ShouldPanic: true,
			Type:        1234,
		},
	}
	for _, c := range cases {
		c.Expected.Target = c.Type

		if c.ShouldPanic {
			assert.Panics(t, func() {
				_ = typeToDescription(c.Type)
			})
		} else {
			assert.Equal(t, c.Expected, typeToDescription(c.Type), "Unexpected descriptor for type: %T", c.Type)
		}
	}
}
