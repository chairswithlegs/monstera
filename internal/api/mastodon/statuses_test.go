package mastodon

import (
	"bytes"
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

func TestStatusesHandler_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, "https://example.com", "example.com", 500, slog.Default())
	logger := slog.Default()
	deps := Deps{Statuses: statusSvc, Accounts: accountSvc, Logger: logger, InstanceDomain: "example.com"}
	handler := NewStatusesHandler(deps)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"status":"hello world"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		var errBody map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&errBody))
		assert.Equal(t, "The access token is invalid", errBody["error"])
	})

	t.Run("blank status returns 422", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"status":"   "}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var errBody map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&errBody))
		assert.Equal(t, "status cannot be blank", errBody["error"])
	})

	t.Run("valid JSON creates status and returns 200", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"status":"Hello from API test"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var statusBody map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&statusBody))
		assert.Contains(t, statusBody["content"], "Hello from API test")
		assert.Equal(t, "bob", statusBody["account"].(map[string]any)["username"])
	})

	t.Run("valid form body creates status", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "charlie",
			Email:        "charlie@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		formBody := bytes.NewBufferString("status=Form+post+content")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", formBody)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var statusBody map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&statusBody))
		assert.Contains(t, statusBody["content"], "Form post content")
	})

}

func TestStatusesHandler_Create_account_without_user_returns_401(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	statusSvc := service.NewStatusService(st, service.NoopFederationPublisher, "https://example.com", "example.com", 500, slog.Default())
	logger := slog.Default()
	deps := Deps{Statuses: statusSvc, Accounts: accountSvc, Logger: logger, InstanceDomain: "example.com"}
	handler := NewStatusesHandler(deps)

	acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "nouser"})
	require.NoError(t, err)

	body := bytes.NewBufferString(`{"status":"orphan account post"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.WithAccount(req.Context(), acc))
	rec := httptest.NewRecorder()
	handler.POSTStatuses(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	var errBody map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&errBody))
	assert.Equal(t, "The access token is invalid", errBody["error"])
}

func TestParseCreateStatusRequest(t *testing.T) {
	t.Parallel()

	t.Run("invalid JSON returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		_, err := parseCreateStatusRequest(req)
		require.Error(t, err)
		assert.Equal(t, "invalid JSON", err.Error())
	})

	t.Run("empty status returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{"status":""}`))
		req.Header.Set("Content-Type", "application/json")
		_, err := parseCreateStatusRequest(req)
		require.Error(t, err)
		assert.Equal(t, "status cannot be blank", err.Error())
	})

	t.Run("valid JSON parses fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{"status":"hi","visibility":"private","spoiler_text":"cw","sensitive":true,"language":"en"}`))
		req.Header.Set("Content-Type", "application/json")
		parsed, err := parseCreateStatusRequest(req)
		require.NoError(t, err)
		assert.Equal(t, "hi", parsed.Status)
		assert.Equal(t, "private", parsed.Visibility)
		assert.Equal(t, "cw", parsed.SpoilerText)
		assert.True(t, parsed.Sensitive)
		assert.Equal(t, "en", parsed.Language)
	})
}
