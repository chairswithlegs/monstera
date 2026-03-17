package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportsHandler_POSTReports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	moderationSvc := service.NewModerationService(st)
	handler := NewReportsHandler(moderationSvc, accountSvc, "example.com")

	reporter, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "reporter",
		Email:    "reporter@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "target"})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := map[string]any{"account_id": target.ID}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTReports(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing account_id returns 422", func(t *testing.T) {
		body := map[string]any{}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", bytes.NewReader(b))
		req = req.WithContext(middleware.WithAccount(req.Context(), reporter))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTReports(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("invalid category returns 422", func(t *testing.T) {
		body := map[string]any{"account_id": target.ID, "category": "invalid"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", bytes.NewReader(b))
		req = req.WithContext(middleware.WithAccount(req.Context(), reporter))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTReports(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("target not found returns 404", func(t *testing.T) {
		body := map[string]any{"account_id": "01nonexistent"}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", bytes.NewReader(b))
		req = req.WithContext(middleware.WithAccount(req.Context(), reporter))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTReports(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("success returns 200 and report", func(t *testing.T) {
		body := map[string]any{
			"account_id": target.ID,
			"comment":    "spam account",
			"category":   "spam",
			"status_ids": []string{},
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", bytes.NewReader(b))
		req = req.WithContext(middleware.WithAccount(req.Context(), reporter))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTReports(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.NotEmpty(t, out["id"])
		assert.Equal(t, "spam", out["category"])
		assert.Equal(t, "spam account", out["comment"])
		assert.False(t, out["action_taken"].(bool))
		assert.False(t, out["forwarded"].(bool))
		assert.Equal(t, target.ID, out["target_account"].(map[string]any)["id"])
	})
}
