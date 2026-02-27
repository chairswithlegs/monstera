package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/verify_credentials", nil)
		rec := httptest.NewRecorder()
		handler.GETVerifyCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
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
		handler.GETVerifyCredentials(rec, req)
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
		handler.GETVerifyCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAccountsHandler_GETAccountStatuses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	timelineSvc := service.NewTimelineService(st)
	handler := NewAccountsHandler(accountSvc, followSvc, timelineSvc, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID+"/statuses", nil)
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountStatuses(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("timeline nil returns 422", func(t *testing.T) {
		handlerNoTimeline := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID+"/statuses", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handlerNoTimeline.GETAccountStatuses(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("authenticated returns 200 and empty or status list", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "charlie",
			Email:        "charlie@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+acc.ID+"/statuses", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", acc.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body)
	})
}

func TestAccountsHandler_GETFollowers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice-followers",
		Email:        "alice-followers@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/followers", nil)
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("target not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/nonexistent-id/followers", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		req = testutil.AddChiURLParam(req, "id", "nonexistent-id")
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("authenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/followers", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowers(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestAccountsHandler_GETFollowing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice-following",
		Email:        "alice-following@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/following", nil)
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowing(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/following", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), alice))
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETFollowing(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestAccountsHandler_BlockUnblock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated POST block returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/block", nil)
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTBlock(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("POST block returns 200 and relationship", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/block", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTBlock(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["blocking"].(bool))
	})

	t.Run("POST unblock returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/unblock", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTUnblock(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.False(t, body["blocking"].(bool))
	})
}

func TestAccountsHandler_MuteUnmute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "bob",
		Email:        "bob@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated POST mute returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/mute", nil)
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTMute(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("POST mute returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/mute", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTMute(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["muting"].(bool))
	})

	t.Run("POST unmute returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/unmute", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTUnmute(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestAccountsHandler_PATCHUpdateCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, nil, nil)
	handler := NewAccountsHandler(accountSvc, followSvc, nil, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/update_credentials", nil)
		rec := httptest.NewRecorder()
		handler.PATCHUpdateCredentials(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated with display_name returns 200", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/update_credentials", bytes.NewBufferString("display_name=Alice+Updated"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.PATCHUpdateCredentials(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "Alice Updated", body["display_name"])
	})
}
