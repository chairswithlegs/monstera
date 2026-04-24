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

func newAccountsRelHandler(st *testutil.FakeStore) (*AccountsHandler, service.AccountService) {
	accountSvc := service.NewAccountService(st, "https://example.com")
	remoteFollowSvc := service.NewRemoteFollowService(st)
	followSvc := service.NewFollowService(st, accountSvc, remoteFollowSvc, nil)
	tagFollowSvc := service.NewTagFollowService(st, 0)
	handler := NewAccountsHandler(accountSvc, followSvc, tagFollowSvc, nil, nil, nil, nil, nil, nil, 0, "example.com")
	return handler, accountSvc
}

func TestAccountsRelationships_POSTFollow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	handler, accountSvc := newAccountsRelHandler(st)

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "actor",
		Email:    "actor@example.com",
		Password: "password123",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	target, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "target",
		Email:    "target@example.com",
		Password: "password123",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/follow", nil)
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTFollow(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts//follow", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.POSTFollow(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("success returns 200 with following true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+target.ID+"/follow", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		req = testutil.AddChiURLParam(req, "id", target.ID)
		rec := httptest.NewRecorder()
		handler.POSTFollow(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.True(t, body["following"].(bool))
	})
}

func TestAccountsRelationships_GETRelationships_NoIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	handler, accountSvc := newAccountsRelHandler(st)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "reluser",
		Email:    "reluser@example.com",
		Password: "password123",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/relationships", nil)
	req = req.WithContext(middleware.WithAccount(req.Context(), acc))
	rec := httptest.NewRecorder()
	handler.GETRelationships(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}
