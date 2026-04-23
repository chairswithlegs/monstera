//go:build integration

package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/chairswithlegs/monstera/internal/uid"
)

func TestIntegration_OAuthStore(t *testing.T) {
	s, ctx := setupTestStore(t)

	t.Run("CreateApplication_GetByClientID", func(t *testing.T) {
		app := createTestOAuthApp(t, s, ctx)

		got, err := s.GetApplicationByClientID(ctx, app.ClientID)
		require.NoError(t, err)
		assert.Equal(t, app.ID, got.ID)
		assert.Equal(t, app.Name, got.Name)
		assert.Equal(t, "read write follow push", got.Scopes)
	})

	t.Run("GetApplicationByClientID_not_found", func(t *testing.T) {
		_, err := s.GetApplicationByClientID(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("AuthorizationCode_create_get_delete", func(t *testing.T) {
		app := createTestOAuthApp(t, s, ctx)
		acc := createTestLocalAccount(t, s, ctx)
		codeID := uid.New()
		codeValue := "authcode_" + uid.New()

		ac, err := s.CreateAuthorizationCode(ctx, store.CreateAuthorizationCodeInput{
			ID:            codeID,
			Code:          codeValue,
			ApplicationID: app.ID,
			AccountID:     acc.ID,
			RedirectURI:   "urn:ietf:wg:oauth:2.0:oob",
			Scopes:        "read write",
			ExpiresAt:     time.Now().Add(10 * time.Minute),
		})
		require.NoError(t, err)
		assert.Equal(t, codeID, ac.ID)
		assert.Equal(t, codeValue, ac.Code)

		got, err := s.GetAuthorizationCode(ctx, codeValue)
		require.NoError(t, err)
		assert.Equal(t, codeID, got.ID)
		assert.Equal(t, app.ID, got.ApplicationID)
		assert.Equal(t, acc.ID, got.AccountID)

		err = s.DeleteAuthorizationCode(ctx, codeValue)
		require.NoError(t, err)

		_, err = s.GetAuthorizationCode(ctx, codeValue)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("AuthorizationCode_with_PKCE", func(t *testing.T) {
		app := createTestOAuthApp(t, s, ctx)
		acc := createTestLocalAccount(t, s, ctx)
		codeID := uid.New()
		codeValue := "pkce_" + uid.New()

		ac, err := s.CreateAuthorizationCode(ctx, store.CreateAuthorizationCodeInput{
			ID:                  codeID,
			Code:                codeValue,
			ApplicationID:       app.ID,
			AccountID:           acc.ID,
			RedirectURI:         "urn:ietf:wg:oauth:2.0:oob",
			Scopes:              "read",
			CodeChallenge:       testutil.StrPtr("challenge123"),
			CodeChallengeMethod: testutil.StrPtr("S256"),
			ExpiresAt:           time.Now().Add(10 * time.Minute),
		})
		require.NoError(t, err)
		require.NotNil(t, ac.CodeChallenge)
		assert.Equal(t, "challenge123", *ac.CodeChallenge)
		require.NotNil(t, ac.CodeChallengeMethod)
		assert.Equal(t, "S256", *ac.CodeChallengeMethod)
	})

	t.Run("GetAuthorizationCode_not_found", func(t *testing.T) {
		_, err := s.GetAuthorizationCode(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("AccessToken_create_get_revoke", func(t *testing.T) {
		app := createTestOAuthApp(t, s, ctx)
		acc := createTestLocalAccount(t, s, ctx)
		tokenID := uid.New()
		tokenValue := "token_" + uid.New()

		tok, err := s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID:            tokenID,
			ApplicationID: app.ID,
			AccountID:     &acc.ID,
			Token:         tokenValue,
			Scopes:        "read write",
		})
		require.NoError(t, err)
		assert.Equal(t, tokenID, tok.ID)
		assert.Equal(t, tokenValue, tok.Token)
		require.NotNil(t, tok.AccountID)
		assert.Equal(t, acc.ID, *tok.AccountID)

		got, err := s.GetAccessToken(ctx, tokenValue)
		require.NoError(t, err)
		assert.Equal(t, tokenID, got.ID)
		assert.Nil(t, got.RevokedAt)

		err = s.RevokeAccessToken(ctx, tokenValue)
		require.NoError(t, err)

		_, err = s.GetAccessToken(ctx, tokenValue)
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("GetAccessToken_not_found", func(t *testing.T) {
		_, err := s.GetAccessToken(ctx, "nonexistent_"+uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("AccessToken_client_credentials_no_account", func(t *testing.T) {
		app := createTestOAuthApp(t, s, ctx)
		tokenID := uid.New()
		tokenValue := "cc_token_" + uid.New()

		tok, err := s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID:            tokenID,
			ApplicationID: app.ID,
			AccountID:     nil,
			Token:         tokenValue,
			Scopes:        "read",
		})
		require.NoError(t, err)
		assert.Nil(t, tok.AccountID)

		got, err := s.GetAccessToken(ctx, tokenValue)
		require.NoError(t, err)
		assert.Nil(t, got.AccountID)
	})

	t.Run("GetApplicationByID", func(t *testing.T) {
		app := createTestOAuthApp(t, s, ctx)

		got, err := s.GetApplicationByID(ctx, app.ID)
		require.NoError(t, err)
		assert.Equal(t, app.ClientID, got.ClientID)

		_, err = s.GetApplicationByID(ctx, uid.New())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("ListAuthorizedApplicationsForAccount_dedup_and_filters", func(t *testing.T) {
		app1 := createTestOAuthApp(t, s, ctx)
		app2 := createTestOAuthApp(t, s, ctx)
		app3 := createTestOAuthApp(t, s, ctx)
		acc := createTestLocalAccount(t, s, ctx)

		// Two active tokens for app1 — expect one row with the newest scopes.
		_, err := s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID: uid.New(), ApplicationID: app1.ID, AccountID: &acc.ID,
			Token: "t1_older_" + uid.New(), Scopes: "read",
		})
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
		_, err = s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID: uid.New(), ApplicationID: app1.ID, AccountID: &acc.ID,
			Token: "t1_newer_" + uid.New(), Scopes: "read write",
		})
		require.NoError(t, err)

		// Revoked token for app2 — should be excluded.
		revokedToken := "t2_revoked_" + uid.New()
		_, err = s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID: uid.New(), ApplicationID: app2.ID, AccountID: &acc.ID,
			Token: revokedToken, Scopes: "read",
		})
		require.NoError(t, err)
		require.NoError(t, s.RevokeAccessToken(ctx, revokedToken))

		// Expired token for app3 — should be excluded.
		expired := time.Now().Add(-1 * time.Hour)
		_, err = s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID: uid.New(), ApplicationID: app3.ID, AccountID: &acc.ID,
			Token: "t3_expired_" + uid.New(), Scopes: "read", ExpiresAt: &expired,
		})
		require.NoError(t, err)

		apps, err := s.ListAuthorizedApplicationsForAccount(ctx, acc.ID)
		require.NoError(t, err)
		require.Len(t, apps, 1)
		assert.Equal(t, app1.ID, apps[0].ApplicationID)
		assert.Equal(t, "read write", apps[0].TokenScopes)
	})

	t.Run("ListAuthorizedApplicationsForAccount_empty", func(t *testing.T) {
		acc := createTestLocalAccount(t, s, ctx)
		apps, err := s.ListAuthorizedApplicationsForAccount(ctx, acc.ID)
		require.NoError(t, err)
		assert.Empty(t, apps)
	})

	t.Run("RevokeAccessTokensForAccountApp_revokes_all_active", func(t *testing.T) {
		app := createTestOAuthApp(t, s, ctx)
		acc := createTestLocalAccount(t, s, ctx)

		t1 := "rev_a_" + uid.New()
		t2 := "rev_b_" + uid.New()
		_, err := s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID: uid.New(), ApplicationID: app.ID, AccountID: &acc.ID, Token: t1, Scopes: "read",
		})
		require.NoError(t, err)
		_, err = s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID: uid.New(), ApplicationID: app.ID, AccountID: &acc.ID, Token: t2, Scopes: "write",
		})
		require.NoError(t, err)

		revoked, err := s.RevokeAccessTokensForAccountApp(ctx, acc.ID, app.ID)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{t1, t2}, revoked)

		_, err = s.GetAccessToken(ctx, t1)
		require.ErrorIs(t, err, domain.ErrNotFound)
		_, err = s.GetAccessToken(ctx, t2)
		require.ErrorIs(t, err, domain.ErrNotFound)

		// Second call is a no-op and returns an empty slice.
		revoked, err = s.RevokeAccessTokensForAccountApp(ctx, acc.ID, app.ID)
		require.NoError(t, err)
		assert.Empty(t, revoked)
	})

	t.Run("RevokeAccessTokensForAccountApp_scoped_to_account", func(t *testing.T) {
		app := createTestOAuthApp(t, s, ctx)
		acc1 := createTestLocalAccount(t, s, ctx)
		acc2 := createTestLocalAccount(t, s, ctx)

		keepToken := "keep_" + uid.New()
		_, err := s.CreateAccessToken(ctx, store.CreateAccessTokenInput{
			ID: uid.New(), ApplicationID: app.ID, AccountID: &acc2.ID, Token: keepToken, Scopes: "read",
		})
		require.NoError(t, err)

		revoked, err := s.RevokeAccessTokensForAccountApp(ctx, acc1.ID, app.ID)
		require.NoError(t, err)
		assert.Empty(t, revoked)

		got, err := s.GetAccessToken(ctx, keepToken)
		require.NoError(t, err)
		assert.Nil(t, got.RevokedAt)
	})
}
