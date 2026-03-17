package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchHandler_GETSearch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	searchSvc := service.NewSearchService(st, nil)
	handler := NewSearchHandler(searchSvc, "example.com")

	t.Run("missing q returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/search", nil)
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Contains(t, body["error"], "q is required")
	})

	t.Run("empty q returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/search?q=", nil)
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("invalid limit returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/search?q=foo&limit=abc", nil)
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("limit zero returns 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/search?q=foo&limit=0", nil)
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("success returns 200 with accounts statuses hashtags", func(t *testing.T) {
		_, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "alice"})
		require.NoError(t, err)
		_, _ = st.GetOrCreateHashtag(ctx, "golang")
		_, _ = st.GetOrCreateHashtag(ctx, "go")

		req := httptest.NewRequest(http.MethodGet, "/api/v2/search?q=alice", nil)
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body struct {
			Accounts []map[string]any `json:"accounts"`
			Statuses []any            `json:"statuses"`
			Hashtags []map[string]any `json:"hashtags"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body.Accounts, 1)
		assert.Equal(t, "alice", body.Accounts[0]["username"])
		assert.Equal(t, "alice", body.Accounts[0]["acct"])
		assert.NotNil(t, body.Statuses)
		assert.Empty(t, body.Statuses)
		assert.NotNil(t, body.Hashtags)
	})

	t.Run("type=accounts only returns accounts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/search?q=x&type=accounts", nil)
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body struct {
			Accounts []any `json:"accounts"`
			Statuses []any `json:"statuses"`
			Hashtags []any `json:"hashtags"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body.Accounts)
		assert.Empty(t, body.Statuses)
		assert.Empty(t, body.Hashtags)
	})

	t.Run("type=hashtags searches hashtags", func(t *testing.T) {
		_, _ = st.GetOrCreateHashtag(ctx, "testtag")
		req := httptest.NewRequest(http.MethodGet, "/api/v2/search?q=test&type=hashtags", nil)
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body struct {
			Hashtags []map[string]any `json:"hashtags"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body.Hashtags, 1)
		assert.Equal(t, "testtag", body.Hashtags[0]["name"])
	})

	t.Run("authenticated viewer is passed to service", func(t *testing.T) {
		acc, err := accountSvc.Create(ctx, service.CreateAccountInput{Username: "bob"})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "/api/v2/search?q=bob", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body struct {
			Accounts []map[string]any `json:"accounts"`
		}
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body.Accounts, 1)
		assert.Equal(t, "bob", body.Accounts[0]["username"])
	})

	t.Run("limit over 40 is clamped", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/search?q=z&limit=100", nil)
		rec := httptest.NewRecorder()
		handler.GETSearch(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
