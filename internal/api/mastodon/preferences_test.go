package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestPreferencesHandler_GETPreferences(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	handler := NewPreferencesHandler(accountSvc)

	account, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/preferences", nil)
		rec := httptest.NewRecorder()
		handler.GETPreferences(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200 with preference keys", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/preferences", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), account))
		rec := httptest.NewRecorder()
		handler.GETPreferences(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body PreferencesResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotEmpty(t, body.PostingDefaultVisibility)
		assert.Contains(t, []string{"public", "unlisted", "private", "direct"}, body.PostingDefaultVisibility)
		assert.Equal(t, "default", body.ReadingExpandMedia)
	})
}
