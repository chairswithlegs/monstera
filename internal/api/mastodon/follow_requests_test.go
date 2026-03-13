package mastodon

import (
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

func TestFollowRequestsHandler_GETFollowRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, service.NewAccountService(st, "https://example.com"))
	handler := NewFollowRequestsHandler(followSvc, accountSvc, "example.com")

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/follow_requests", nil)
		rec := httptest.NewRecorder()
		handler.GETFollowRequests(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated returns 200 and array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/follow_requests", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETFollowRequests(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})
}

func TestFollowRequestsHandler_POSTAuthorize(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, service.NewAccountService(st, "https://example.com"))
	handler := NewFollowRequestsHandler(followSvc, accountSvc, "example.com")

	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "target",
		Email:        "target@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	requester, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "requester"})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/follow_requests/"+requester.ID+"/authorize", nil)
		req = testutil.AddChiURLParam(req, "id", requester.ID)
		rec := httptest.NewRecorder()
		handler.POSTAuthorize(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/follow_requests/01nonexistent/authorize", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), target))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.POSTAuthorize(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestFollowRequestsHandler_POSTReject(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	followSvc := service.NewFollowService(st, service.NewAccountService(st, "https://example.com"))
	handler := NewFollowRequestsHandler(followSvc, accountSvc, "example.com")

	ctx := context.Background()
	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "target",
		Email:        "target@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)
	requester, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "requester"})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/follow_requests/"+requester.ID+"/reject", nil)
		req = testutil.AddChiURLParam(req, "id", requester.ID)
		rec := httptest.NewRecorder()
		handler.POSTReject(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/follow_requests/01nonexistent/reject", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), target))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.POSTReject(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
