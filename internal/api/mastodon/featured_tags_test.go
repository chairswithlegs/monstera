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
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeaturedTagsHandler_CRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	featuredTagSvc := service.NewFeaturedTagService(st)
	handler := NewFeaturedTagsHandler(featuredTagSvc, accountSvc, "example.com")

	actor, err := accountSvc.Register(ctx, service.RegisterInput{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("GET unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/featured_tags", nil)
		rec := httptest.NewRecorder()
		handler.GETFeaturedTags(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("GET authenticated empty returns 200 and empty array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/featured_tags", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETFeaturedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("POST with name returns 200 and featured tag", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/featured_tags", strings.NewReader(`{"name":"golang"}`))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.POSTFeaturedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, "golang", body["name"])
		assert.NotEmpty(t, body["id"])
		assert.Contains(t, body["url"], "/tagged/golang")
	})

	t.Run("GET after create returns featured tag in list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/featured_tags", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETFeaturedTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.GreaterOrEqual(t, len(body), 1)
		var found bool
		for _, ft := range body {
			if ft["name"] == "golang" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("DELETE by id returns 200", func(t *testing.T) {
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/featured_tags", nil)
		listReq = listReq.WithContext(middleware.WithAccount(listReq.Context(), actor))
		listRec := httptest.NewRecorder()
		handler.GETFeaturedTags(listRec, listReq)
		var list []map[string]any
		require.NoError(t, json.NewDecoder(listRec.Body).Decode(&list))
		require.GreaterOrEqual(t, len(list), 1)
		id, _ := list[0]["id"].(string)
		require.NotEmpty(t, id)

		delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/featured_tags/"+id, nil)
		delReq = delReq.WithContext(middleware.WithAccount(delReq.Context(), actor))
		delReq = testutil.AddChiURLParam(delReq, "id", id)
		delRec := httptest.NewRecorder()
		handler.DELETEFeaturedTag(delRec, delReq)
		assert.Equal(t, http.StatusOK, delRec.Code)
	})

	t.Run("GET suggestions returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/featured_tags/suggestions", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), actor))
		rec := httptest.NewRecorder()
		handler.GETFeaturedTagSuggestions(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body)
	})
}
