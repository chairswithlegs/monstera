package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimelinesHandler_Home(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	timelineSvc := service.NewTimelineService(st)
	handler := NewTimelinesHandler(timelineSvc, "example.com")

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/timelines/home", nil)
		rec := httptest.NewRecorder()
		handler.GETHome(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated with empty timeline returns 200 and empty array", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "alice",
			Email:        "alice@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/timelines/home", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETHome(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("authenticated with statuses returns 200 and status list", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		statusText := "Hello timeline"
		_, err = st.CreateStatus(ctx, store.CreateStatusInput{
			ID:         uid.New(),
			URI:        "https://example.com/statuses/01",
			AccountID:  acc.ID,
			Text:       testutil.StrPtr(statusText),
			Content:    testutil.StrPtr("<p>Hello timeline</p>"),
			Visibility: domain.VisibilityPublic,
			APID:       "https://example.com/statuses/01",
			Local:      true,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/timelines/home", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETHome(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Contains(t, body[0]["content"], statusText)
		assert.Equal(t, "bob", body[0]["account"].(map[string]any)["username"])
	})

	t.Run("with max_id and limit query params", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username:     "charlie",
			Email:        "charlie@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/timelines/home?max_id=01H&limit=5", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETHome(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body)
	})
}

func TestTimelinesHandler_GETTag(t *testing.T) {
	t.Parallel()
	st := testutil.NewFakeStore()
	timelineSvc := service.NewTimelineService(st)
	handler := NewTimelinesHandler(timelineSvc, "example.com")

	t.Run("empty hashtag returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/timelines/tag/", nil)
		req = testutil.AddChiURLParam(req, "hashtag", "")
		rec := httptest.NewRecorder()
		handler.GETTag(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("returns 200 and empty or status list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/timelines/tag/foo", nil)
		req = testutil.AddChiURLParam(req, "hashtag", "foo")
		rec := httptest.NewRecorder()
		handler.GETTag(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.NotNil(t, body)
	})
}
