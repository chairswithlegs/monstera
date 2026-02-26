package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"

	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountsHandler_VerifyCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	logger := slog.Default()
	handler := NewAccountsHandler(accountSvc, logger, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/verify_credentials", nil)
		rec := httptest.NewRecorder()
		handler.VerifyCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "The access token is invalid", body["error"])
	})

	t.Run("authenticated with valid account returns 200 and account", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/verify_credentials", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.VerifyCredentials(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "alice", body["username"])
		assert.Equal(t, "alice", body["acct"])
	})

	t.Run("account in context but not in store returns 401", func(t *testing.T) {
		orphan := &domain.Account{ID: "01nonexistent", Username: "orphan"}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/verify_credentials", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), orphan))
		rec := httptest.NewRecorder()
		handler.VerifyCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "The access token is invalid", body["error"])
	})
}
