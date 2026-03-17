package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPushTestHandler(t *testing.T) (*PushHandler, *domain.Account, *oauth.TokenClaims) {
	t.Helper()
	st := testutil.NewFakeStore()
	pushSvc := service.NewPushSubscriptionService(st)
	handler := NewPushHandler(pushSvc, "test-vapid-key")

	accountSvc := service.NewAccountService(st, "https://example.com")
	actor, err := accountSvc.Register(context.Background(), service.RegisterInput{
		Username: "alice", Email: "alice@example.com", Password: "hash", Role: domain.RoleUser,
	})
	require.NoError(t, err)

	claims := &oauth.TokenClaims{
		AccessTokenID: "tok-123",
		AccountID:     actor.ID,
		ApplicationID: "app-1",
		Scopes:        oauth.Parse("push"),
	}
	return handler, actor, claims
}

func TestPushHandler_POSTSubscription(t *testing.T) {
	t.Parallel()
	handler, actor, claims := newPushTestHandler(t)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := `{"subscription":{"endpoint":"https://push.example.com","keys":{"p256dh":"key","auth":"secret"}}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/push/subscription", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTSubscription(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("valid creates subscription", func(t *testing.T) {
		body := `{"subscription":{"endpoint":"https://push.example.com","keys":{"p256dh":"key","auth":"secret"}},"data":{"alerts":{"follow":true,"mention":true},"policy":"all"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/push/subscription", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := middleware.WithAccount(req.Context(), actor)
		ctx = middleware.WithTokenClaims(ctx, claims)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.POSTSubscription(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, "https://push.example.com", resp["endpoint"])
		assert.Equal(t, "test-vapid-key", resp["server_key"])
		assert.NotEmpty(t, resp["id"])
		alerts := resp["alerts"].(map[string]any)
		assert.True(t, alerts["follow"].(bool))
		assert.True(t, alerts["mention"].(bool))
	})
}

func TestPushHandler_GETSubscription(t *testing.T) {
	t.Parallel()
	handler, actor, claims := newPushTestHandler(t)

	t.Run("not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/push/subscription", nil)
		ctx := middleware.WithAccount(req.Context(), actor)
		ctx = middleware.WithTokenClaims(ctx, claims)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.GETSubscription(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns existing subscription", func(t *testing.T) {
		body := `{"subscription":{"endpoint":"https://push.example.com","keys":{"p256dh":"k","auth":"a"}}}`
		createReq := httptest.NewRequest(http.MethodPost, "/api/v1/push/subscription", strings.NewReader(body))
		createReq.Header.Set("Content-Type", "application/json")
		ctx := middleware.WithAccount(createReq.Context(), actor)
		ctx = middleware.WithTokenClaims(ctx, claims)
		createReq = createReq.WithContext(ctx)
		createRec := httptest.NewRecorder()
		handler.POSTSubscription(createRec, createReq)
		require.Equal(t, http.StatusOK, createRec.Code)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/push/subscription", nil)
		ctx = middleware.WithAccount(req.Context(), actor)
		ctx = middleware.WithTokenClaims(ctx, claims)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.GETSubscription(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, "https://push.example.com", resp["endpoint"])
	})
}

func TestPushHandler_DELETESubscription(t *testing.T) {
	t.Parallel()
	handler, actor, claims := newPushTestHandler(t)

	body := `{"subscription":{"endpoint":"https://push.example.com","keys":{"p256dh":"k","auth":"a"}}}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/push/subscription", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	ctx := middleware.WithAccount(createReq.Context(), actor)
	ctx = middleware.WithTokenClaims(ctx, claims)
	createReq = createReq.WithContext(ctx)
	createRec := httptest.NewRecorder()
	handler.POSTSubscription(createRec, createReq)
	require.Equal(t, http.StatusOK, createRec.Code)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/push/subscription", nil)
	ctx = middleware.WithAccount(req.Context(), actor)
	ctx = middleware.WithTokenClaims(ctx, claims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.DELETESubscription(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/push/subscription", nil)
	ctx = middleware.WithAccount(getReq.Context(), actor)
	ctx = middleware.WithTokenClaims(ctx, claims)
	getReq = getReq.WithContext(ctx)
	getRec := httptest.NewRecorder()
	handler.GETSubscription(getRec, getReq)
	assert.Equal(t, http.StatusNotFound, getRec.Code)
}
