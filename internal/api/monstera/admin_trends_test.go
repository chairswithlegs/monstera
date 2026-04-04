package monstera

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTrendingLinkDenylistService is a minimal TrendingLinkDenylistService for testing.
type fakeTrendingLinkDenylistService struct {
	denylist []string
	err      error
}

func (f *fakeTrendingLinkDenylistService) GetDenylist(_ context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.denylist, nil
}

func (f *fakeTrendingLinkDenylistService) AddDenylist(_ context.Context, url string) error {
	if f.err != nil {
		return f.err
	}
	f.denylist = append(f.denylist, url)
	return nil
}

func (f *fakeTrendingLinkDenylistService) RemoveDenylist(_ context.Context, url string) error {
	if f.err != nil {
		return f.err
	}
	for i, u := range f.denylist {
		if u == url {
			f.denylist = append(f.denylist[:i], f.denylist[i+1:]...)
			return nil
		}
	}
	return nil
}

func TestAdminTrendsHandler_GETDenylist_empty(t *testing.T) {
	t.Parallel()
	handler := NewAdminTrendsHandler(&fakeTrendingLinkDenylistService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/trends/denylist", nil)
	rec := httptest.NewRecorder()
	handler.GETDenylist(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string][]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body["urls"])
}

func TestAdminTrendsHandler_GETDenylist_withData(t *testing.T) {
	t.Parallel()
	svc := &fakeTrendingLinkDenylistService{denylist: []string{"https://spam.example/a", "https://spam.example/b"}}
	handler := NewAdminTrendsHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/trends/denylist", nil)
	rec := httptest.NewRecorder()
	handler.GETDenylist(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string][]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, []string{"https://spam.example/a", "https://spam.example/b"}, body["urls"])
}

func TestAdminTrendsHandler_POSTDenylist_valid(t *testing.T) {
	t.Parallel()
	svc := &fakeTrendingLinkDenylistService{}
	handler := NewAdminTrendsHandler(svc)

	b, _ := json.Marshal(map[string]string{"url": "https://spam.example/bad"})
	req := httptest.NewRequest(http.MethodPost, "/admin/trends/denylist", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.POSTDenylist(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"https://spam.example/bad"}, svc.denylist)
}

func TestAdminTrendsHandler_POSTDenylist_missingURL(t *testing.T) {
	t.Parallel()
	handler := NewAdminTrendsHandler(&fakeTrendingLinkDenylistService{})

	b, _ := json.Marshal(map[string]string{"url": ""})
	req := httptest.NewRequest(http.MethodPost, "/admin/trends/denylist", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.POSTDenylist(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestAdminTrendsHandler_DELETEDenylist_valid(t *testing.T) {
	t.Parallel()
	svc := &fakeTrendingLinkDenylistService{denylist: []string{"https://spam.example/bad"}}
	handler := NewAdminTrendsHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/admin/trends/denylist?url=https%3A%2F%2Fspam.example%2Fbad", nil)
	rec := httptest.NewRecorder()
	handler.DELETEDenylist(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, svc.denylist)
}

func TestAdminTrendsHandler_DELETEDenylist_urlWithPath(t *testing.T) {
	t.Parallel()
	// Verify that URLs with path components work via the query param approach.
	svc := &fakeTrendingLinkDenylistService{denylist: []string{"https://example.com/some/deep/path"}}
	handler := NewAdminTrendsHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/admin/trends/denylist?url=https%3A%2F%2Fexample.com%2Fsome%2Fdeep%2Fpath", nil)
	rec := httptest.NewRecorder()
	handler.DELETEDenylist(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, svc.denylist)
}

func TestAdminTrendsHandler_DELETEDenylist_missingURL(t *testing.T) {
	t.Parallel()
	handler := NewAdminTrendsHandler(&fakeTrendingLinkDenylistService{})

	req := httptest.NewRequest(http.MethodDelete, "/admin/trends/denylist", nil)
	rec := httptest.NewRecorder()
	handler.DELETEDenylist(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
