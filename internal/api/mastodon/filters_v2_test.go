package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func newFiltersV2Handler(t *testing.T) (*FiltersV2Handler, service.UserFilterV2Service) {
	t.Helper()
	fake := testutil.NewFakeStore()
	svc := service.NewUserFilterV2Service(fake)
	return NewFiltersV2Handler(svc), svc
}

func TestFiltersV2Handler_Unauthorized(t *testing.T) {
	t.Parallel()
	h, _ := newFiltersV2Handler(t)

	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{"GETFiltersV2", "GET", "/api/v2/filters", h.GETFiltersV2},
		{"POSTFiltersV2", "POST", "/api/v2/filters", h.POSTFiltersV2},
		{"GETFilterV2", "GET", "/api/v2/filters/id", h.GETFilterV2},
		{"PUTFilterV2", "PUT", "/api/v2/filters/id", h.PUTFilterV2},
		{"DELETEFilterV2", "DELETE", "/api/v2/filters/id", h.DELETEFilterV2},
		{"GETFilterKeywords", "GET", "/api/v2/filters/id/keywords", h.GETFilterKeywords},
		{"POSTFilterKeyword", "POST", "/api/v2/filters/id/keywords", h.POSTFilterKeyword},
		{"GETFilterKeyword", "GET", "/api/v2/filter_keywords/id", h.GETFilterKeyword},
		{"PUTFilterKeyword", "PUT", "/api/v2/filter_keywords/id", h.PUTFilterKeyword},
		{"DELETEFilterKeyword", "DELETE", "/api/v2/filter_keywords/id", h.DELETEFilterKeyword},
		{"GETFilterStatuses", "GET", "/api/v2/filters/id/statuses", h.GETFilterStatuses},
		{"POSTFilterStatus", "POST", "/api/v2/filters/id/statuses", h.POSTFilterStatus},
		{"GETFilterStatus", "GET", "/api/v2/filter_statuses/id", h.GETFilterStatus},
		{"DELETEFilterStatus", "DELETE", "/api/v2/filter_statuses/id", h.DELETEFilterStatus},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			tc.handler(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestFiltersV2Handler_GETFiltersV2(t *testing.T) {
	t.Parallel()
	h, svc := newFiltersV2Handler(t)
	account := &domain.Account{ID: "acc1"}
	ctx := middleware.WithAccount(context.Background(), account)

	// Create a filter
	f, err := svc.CreateFilter(ctx, account.ID, "test filter", []string{"home"}, nil, "warn")
	require.NoError(t, err)
	require.NotNil(t, f)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/filters", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.GETFiltersV2(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Len(t, out, 1)
	assert.Equal(t, f.ID, out[0]["id"])
	assert.Equal(t, "test filter", out[0]["title"])
	assert.Equal(t, "warn", out[0]["filter_action"])
}

func TestFiltersV2Handler_POSTFiltersV2(t *testing.T) {
	t.Parallel()
	h, _ := newFiltersV2Handler(t)
	account := &domain.Account{ID: "acc1"}
	ctx := middleware.WithAccount(context.Background(), account)

	body, _ := json.Marshal(map[string]any{
		"title":         "my filter",
		"context":       []string{"home", "public"},
		"filter_action": "warn",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v2/filters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.POSTFiltersV2(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "my filter", out["title"])
	assert.Equal(t, "warn", out["filter_action"])
}

func TestFiltersV2Handler_POSTFiltersV2_MissingTitle(t *testing.T) {
	t.Parallel()
	h, _ := newFiltersV2Handler(t)
	account := &domain.Account{ID: "acc1"}
	ctx := middleware.WithAccount(context.Background(), account)

	body, _ := json.Marshal(map[string]any{"context": []string{"home"}})
	req := httptest.NewRequest(http.MethodPost, "/api/v2/filters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.POSTFiltersV2(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestFiltersV2Handler_GETFilterV2(t *testing.T) {
	t.Parallel()
	h, svc := newFiltersV2Handler(t)
	account := &domain.Account{ID: "acc1"}
	ctx := middleware.WithAccount(context.Background(), account)

	f, err := svc.CreateFilter(ctx, account.ID, "get me", []string{"home"}, nil, "hide")
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Get("/api/v2/filters/{id}", h.GETFilterV2)
	req := httptest.NewRequest(http.MethodGet, "/api/v2/filters/"+f.ID, nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, f.ID, out["id"])
	assert.Equal(t, "hide", out["filter_action"])
}

func TestFiltersV2Handler_GETFilterV2_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFiltersV2Handler(t)
	account := &domain.Account{ID: "acc1"}
	ctx := middleware.WithAccount(context.Background(), account)

	r := chi.NewRouter()
	r.Get("/api/v2/filters/{id}", h.GETFilterV2)
	req := httptest.NewRequest(http.MethodGet, "/api/v2/filters/nonexistent", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFiltersV2Handler_DELETEFilterV2(t *testing.T) {
	t.Parallel()
	h, svc := newFiltersV2Handler(t)
	account := &domain.Account{ID: "acc1"}
	ctx := middleware.WithAccount(context.Background(), account)

	f, err := svc.CreateFilter(ctx, account.ID, "delete me", []string{"home"}, nil, "warn")
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Delete("/api/v2/filters/{id}", h.DELETEFilterV2)
	req := httptest.NewRequest(http.MethodDelete, "/api/v2/filters/"+f.ID, nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFiltersV2Handler_KeywordCRUD(t *testing.T) {
	t.Parallel()
	h, svc := newFiltersV2Handler(t)
	account := &domain.Account{ID: "acc1"}
	ctx := middleware.WithAccount(context.Background(), account)

	f, err := svc.CreateFilter(ctx, account.ID, "kw filter", []string{"home"}, nil, "warn")
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Get("/api/v2/filters/{id}/keywords", h.GETFilterKeywords)
	r.Post("/api/v2/filters/{id}/keywords", h.POSTFilterKeyword)
	r.Get("/api/v2/filter_keywords/{id}", h.GETFilterKeyword)
	r.Delete("/api/v2/filter_keywords/{id}", h.DELETEFilterKeyword)

	// List keywords (empty)
	req := httptest.NewRequest(http.MethodGet, "/api/v2/filters/"+f.ID+"/keywords", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var kws []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &kws))
	assert.Empty(t, kws)

	// Add keyword
	kwBody, _ := json.Marshal(map[string]any{"keyword": "badword", "whole_word": true})
	req = httptest.NewRequest(http.MethodPost, "/api/v2/filters/"+f.ID+"/keywords", bytes.NewReader(kwBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var kw map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &kw))
	assert.Equal(t, "badword", kw["keyword"])
	assert.Equal(t, true, kw["whole_word"])
	kwID := kw["id"].(string)

	// Get keyword
	req = httptest.NewRequest(http.MethodGet, "/api/v2/filter_keywords/"+kwID, nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Delete keyword
	req = httptest.NewRequest(http.MethodDelete, "/api/v2/filter_keywords/"+kwID, nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFiltersV2Handler_FilterStatusCRUD(t *testing.T) {
	t.Parallel()
	h, svc := newFiltersV2Handler(t)
	account := &domain.Account{ID: "acc1"}
	ctx := middleware.WithAccount(context.Background(), account)

	f, err := svc.CreateFilter(ctx, account.ID, "status filter", []string{"home"}, nil, "warn")
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Get("/api/v2/filters/{id}/statuses", h.GETFilterStatuses)
	r.Post("/api/v2/filters/{id}/statuses", h.POSTFilterStatus)
	r.Get("/api/v2/filter_statuses/{id}", h.GETFilterStatus)
	r.Delete("/api/v2/filter_statuses/{id}", h.DELETEFilterStatus)

	// List statuses (empty)
	req := httptest.NewRequest(http.MethodGet, "/api/v2/filters/"+f.ID+"/statuses", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var fsts []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fsts))
	assert.Empty(t, fsts)

	// Add status
	fsBody, _ := json.Marshal(map[string]any{"status_id": "st123"})
	req = httptest.NewRequest(http.MethodPost, "/api/v2/filters/"+f.ID+"/statuses", bytes.NewReader(fsBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var fs map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fs))
	assert.Equal(t, "st123", fs["status_id"])
	fsID := fs["id"].(string)

	// Delete filter status
	req = httptest.NewRequest(http.MethodDelete, "/api/v2/filter_statuses/"+fsID, nil)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
