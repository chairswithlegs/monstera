package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountsHandler_GETAccountFeaturedTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	featuredTagSvc := service.NewFeaturedTagService(st)
	handler := NewAccountsHandler(accountSvc, nil, nil, nil, nil, nil, nil, nil, featuredTagSvc, 0, "example.com")

	alice, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice-ftags",
		Email:    "alice-ftags@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unknown account returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/nonexistent/featured_tags", nil)
		req = testutil.AddChiURLParam(req, "id", "nonexistent")
		rec := httptest.NewRecorder()
		handler.GETAccountFeaturedTags(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("account with no featured tags returns empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/featured_tags", nil)
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountFeaturedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("account with featured tags returns correct fields", func(t *testing.T) {
		ft, err := featuredTagSvc.CreateFeaturedTag(ctx, alice.ID, "golang")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+alice.ID+"/featured_tags", nil)
		req = testutil.AddChiURLParam(req, "id", alice.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountFeaturedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, ft.ID, body[0]["id"])
		assert.Equal(t, "golang", body[0]["name"])
		assert.Equal(t, "https://example.com/@alice-ftags/tagged/golang", body[0]["url"])
		assert.EqualValues(t, 0, body[0]["statuses_count"])
		assert.Nil(t, body[0]["last_status_at"])
	})

	t.Run("suspended account returns empty array", func(t *testing.T) {
		suspended, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "suspended-ftags",
			Email:    "suspended-ftags@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		_, err = featuredTagSvc.CreateFeaturedTag(ctx, suspended.ID, "golang")
		require.NoError(t, err)
		require.NoError(t, st.SuspendAccount(ctx, suspended.ID))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+suspended.ID+"/featured_tags", nil)
		req = testutil.AddChiURLParam(req, "id", suspended.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountFeaturedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("remote account tag URL uses remote domain", func(t *testing.T) {
		bob, err := accountSvc.CreateOrUpdateRemoteAccount(ctx, service.CreateOrUpdateRemoteInput{
			APID:      "https://mastodon.social/users/bob",
			Username:  "bob",
			Domain:    "mastodon.social",
			PublicKey: "testkey",
			InboxURL:  "https://mastodon.social/users/bob/inbox",
		})
		require.NoError(t, err)
		_, err = featuredTagSvc.CreateFeaturedTag(ctx, bob.ID, "rust")
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+bob.ID+"/featured_tags", nil)
		req = testutil.AddChiURLParam(req, "id", bob.ID)
		rec := httptest.NewRecorder()
		handler.GETAccountFeaturedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, "https://mastodon.social/@bob/tagged/rust", body[0]["url"])
	})
}
