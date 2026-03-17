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
}
