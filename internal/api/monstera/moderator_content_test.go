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

func TestModeratorContentHandler_GETTrendingLinkFilters_empty(t *testing.T) {
	t.Parallel()
	handler := NewModeratorContentHandler(&fakeTrendingLinkDenylistService{})

	req := httptest.NewRequest(http.MethodGet, "/moderator/content/trending-link-filters", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendingLinkFilters(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string][]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body["urls"])
}

func TestModeratorContentHandler_GETTrendingLinkFilters_withData(t *testing.T) {
	t.Parallel()
	svc := &fakeTrendingLinkDenylistService{denylist: []string{"https://spam.example/a", "https://spam.example/b"}}
	handler := NewModeratorContentHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/moderator/content/trending-link-filters", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendingLinkFilters(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string][]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, []string{"https://spam.example/a", "https://spam.example/b"}, body["urls"])
}

func TestModeratorContentHandler_POSTTrendingLinkFilters_valid(t *testing.T) {
	t.Parallel()
	svc := &fakeTrendingLinkDenylistService{}
	handler := NewModeratorContentHandler(svc)

	b, _ := json.Marshal(map[string]string{"url": "https://spam.example/bad"})
	req := httptest.NewRequest(http.MethodPost, "/moderator/content/trending-link-filters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.POSTTrendingLinkFilters(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"https://spam.example/bad"}, svc.denylist)
}

func TestModeratorContentHandler_POSTTrendingLinkFilters_missingURL(t *testing.T) {
	t.Parallel()
	handler := NewModeratorContentHandler(&fakeTrendingLinkDenylistService{})

	b, _ := json.Marshal(map[string]string{"url": ""})
	req := httptest.NewRequest(http.MethodPost, "/moderator/content/trending-link-filters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.POSTTrendingLinkFilters(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestModeratorContentHandler_DELETETrendingLinkFilter_valid(t *testing.T) {
	t.Parallel()
	svc := &fakeTrendingLinkDenylistService{denylist: []string{"https://spam.example/bad"}}
	handler := NewModeratorContentHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/moderator/content/trending-link-filters?url=https%3A%2F%2Fspam.example%2Fbad", nil)
	rec := httptest.NewRecorder()
	handler.DELETETrendingLinkFilter(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, svc.denylist)
}

func TestModeratorContentHandler_DELETETrendingLinkFilter_missingURL(t *testing.T) {
	t.Parallel()
	handler := NewModeratorContentHandler(&fakeTrendingLinkDenylistService{})

	req := httptest.NewRequest(http.MethodDelete, "/moderator/content/trending-link-filters", nil)
	rec := httptest.NewRecorder()
	handler.DELETETrendingLinkFilter(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
