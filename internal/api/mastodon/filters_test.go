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

func TestFiltersHandler_GETFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	filterSvc := service.NewUserFilterService(st)
	handler := NewFiltersHandler(filterSvc)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/filters", nil)
		rec := httptest.NewRecorder()
		handler.GETFilters(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("authenticated empty list returns 200 and empty array", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "alice",
			Email:    "alice@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/filters", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETFilters(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Empty(t, body)
	})

	t.Run("authenticated with filters returns 200 and filter array", func(t *testing.T) {
		acc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "bob",
			Email:    "bob@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)

		_, err = filterSvc.CreateFilter(ctx, acc.ID, "spam", []string{domain.FilterContextHome}, false, nil, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/filters", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.GETFilters(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		require.Len(t, body, 1)
		assert.Equal(t, "spam", body[0]["phrase"])
		assert.Equal(t, []any{domain.FilterContextHome}, body[0]["context"])
		assert.False(t, body[0]["whole_word"].(bool))
		assert.False(t, body[0]["irreversible"].(bool))
	})
}

func TestFiltersHandler_GETFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	filterSvc := service.NewUserFilterService(st)
	handler := NewFiltersHandler(filterSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/filters/01abc", nil)
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.GETFilter(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/filters/", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.GETFilter(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("filter not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/filters/01nonexistent", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.GETFilter(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("filter exists for account returns 200 and filter", func(t *testing.T) {
		f, err := filterSvc.CreateFilter(ctx, acc.ID, "blocked", []string{domain.FilterContextHome, domain.FilterContextNotifications}, true, nil, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/filters/"+f.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", f.ID)
		rec := httptest.NewRecorder()
		handler.GETFilter(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, f.ID, body["id"])
		assert.Equal(t, "blocked", body["phrase"])
		assert.True(t, body["whole_word"].(bool))
	})

	t.Run("filter exists for another account returns 404", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "other",
			Email:    "other@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		f, err := filterSvc.CreateFilter(ctx, otherAcc.ID, "their-filter", []string{domain.FilterContextHome}, false, nil, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/filters/"+f.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", f.ID)
		rec := httptest.NewRecorder()
		handler.GETFilter(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestFiltersHandler_POSTFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	filterSvc := service.NewUserFilterService(st)
	handler := NewFiltersHandler(filterSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"phrase":"test"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/filters", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.POSTFilters(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/filters", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTFilters(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("empty phrase returns 422", func(t *testing.T) {
		body := bytes.NewBufferString(`{"phrase":""}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/filters", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTFilters(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("valid body returns 200 and filter", func(t *testing.T) {
		body := bytes.NewBufferString(`{"phrase":"muted","context":["home","notifications"],"whole_word":true,"irreversible":false}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/filters", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		rec := httptest.NewRecorder()
		handler.POSTFilters(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.NotEmpty(t, out["id"])
		assert.Equal(t, "muted", out["phrase"])
		assert.Equal(t, []any{"home", "notifications"}, out["context"])
		assert.True(t, out["whole_word"].(bool))
		assert.False(t, out["irreversible"].(bool))
	})
}

func TestFiltersHandler_PUTFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	filterSvc := service.NewUserFilterService(st)
	handler := NewFiltersHandler(filterSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		body := bytes.NewBufferString(`{"phrase":"updated"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/filters/01abc", body)
		req.Header.Set("Content-Type", "application/json")
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.PUTFilter(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"phrase":"updated"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/filters/", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.PUTFilter(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		body := bytes.NewBufferString(`{invalid`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/filters/01abc", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.PUTFilter(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("filter not found returns 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"phrase":"updated"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/filters/01nonexistent", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.PUTFilter(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("filter for another account returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "other",
			Email:    "other@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		f, err := filterSvc.CreateFilter(ctx, otherAcc.ID, "their-filter", []string{domain.FilterContextHome}, false, nil, false)
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"phrase":"hacked"}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/filters/"+f.ID, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", f.ID)
		rec := httptest.NewRecorder()
		handler.PUTFilter(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("filter exists for account returns 200 and updated filter", func(t *testing.T) {
		f, err := filterSvc.CreateFilter(ctx, acc.ID, "original", []string{domain.FilterContextHome}, false, nil, false)
		require.NoError(t, err)

		body := bytes.NewBufferString(`{"phrase":"updated-phrase","context":["public"],"whole_word":true,"irreversible":true}`)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/filters/"+f.ID, body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", f.ID)
		rec := httptest.NewRecorder()
		handler.PUTFilter(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var out map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&out))
		assert.Equal(t, f.ID, out["id"])
		assert.Equal(t, "updated-phrase", out["phrase"])
		assert.Equal(t, []any{"public"}, out["context"])
		assert.True(t, out["whole_word"].(bool))
		assert.True(t, out["irreversible"].(bool))
	})
}

func TestFiltersHandler_DELETEFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testutil.NewFakeStore()
	accountSvc := service.NewAccountService(st, "https://example.com")
	filterSvc := service.NewUserFilterService(st)
	handler := NewFiltersHandler(filterSvc)

	acc, err := accountSvc.Register(ctx, service.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "hash",
		Role:     domain.RoleUser,
	})
	require.NoError(t, err)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/filters/01abc", nil)
		req = testutil.AddChiURLParam(req, "id", "01abc")
		rec := httptest.NewRecorder()
		handler.DELETEFilter(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/filters/", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "")
		rec := httptest.NewRecorder()
		handler.DELETEFilter(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("filter not found returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/filters/01nonexistent", nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", "01nonexistent")
		rec := httptest.NewRecorder()
		handler.DELETEFilter(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("filter for another account returns 403", func(t *testing.T) {
		otherAcc, err := accountSvc.Register(ctx, service.RegisterInput{
			Username: "other",
			Email:    "other@example.com",
			Password: "hash",
			Role:     domain.RoleUser,
		})
		require.NoError(t, err)
		f, err := filterSvc.CreateFilter(ctx, otherAcc.ID, "their-filter", []string{domain.FilterContextHome}, false, nil, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/filters/"+f.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", f.ID)
		rec := httptest.NewRecorder()
		handler.DELETEFilter(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("filter for account returns 200", func(t *testing.T) {
		f, err := filterSvc.CreateFilter(ctx, acc.ID, "to-delete", []string{domain.FilterContextHome}, false, nil, false)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/filters/"+f.ID, nil)
		req = req.WithContext(middleware.WithAccount(req.Context(), acc))
		req = testutil.AddChiURLParam(req, "id", f.ID)
		rec := httptest.NewRecorder()
		handler.DELETEFilter(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Body.Bytes())

		_, err = filterSvc.GetFilter(ctx, acc.ID, f.ID)
		assert.Error(t, err)
	})
}
