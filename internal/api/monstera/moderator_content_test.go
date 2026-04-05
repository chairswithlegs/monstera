package monstera

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTrendsService is a minimal TrendsService for testing the moderator content handler.
type fakeTrendsService struct {
	filters []string
	err     error
}

func (f *fakeTrendsService) TrendingStatuses(_ context.Context, _, _ int) ([]service.EnrichedStatus, error) {
	return nil, nil
}

func (f *fakeTrendsService) TrendingTags(_ context.Context, _, _ int) ([]domain.TrendingTag, error) {
	return nil, nil
}

func (f *fakeTrendsService) TrendingLinks(_ context.Context, _, _ int) ([]domain.TrendingLink, error) {
	return nil, nil
}

func (f *fakeTrendsService) RefreshIndexes(_ context.Context) error {
	return nil
}

func (f *fakeTrendsService) ListTrendingLinkFilters(_ context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.filters, nil
}

func (f *fakeTrendsService) AddTrendingLinkFilter(_ context.Context, url string) error {
	if f.err != nil {
		return f.err
	}
	f.filters = append(f.filters, url)
	return nil
}

func (f *fakeTrendsService) RemoveTrendingLinkFilter(_ context.Context, url string) error {
	if f.err != nil {
		return f.err
	}
	for i, u := range f.filters {
		if u == url {
			f.filters = append(f.filters[:i], f.filters[i+1:]...)
			return nil
		}
	}
	return nil
}

func TestModeratorContentHandler_GETTrendingLinkFilters_empty(t *testing.T) {
	t.Parallel()
	handler := NewModeratorContentHandler(&fakeTrendsService{})

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
	svc := &fakeTrendsService{filters: []string{"https://spam.example/a", "https://spam.example/b"}}
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
	svc := &fakeTrendsService{}
	handler := NewModeratorContentHandler(svc)

	b, _ := json.Marshal(map[string]string{"url": "https://spam.example/bad"})
	req := httptest.NewRequest(http.MethodPost, "/moderator/content/trending-link-filters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.POSTTrendingLinkFilters(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"https://spam.example/bad"}, svc.filters)
}

func TestModeratorContentHandler_POSTTrendingLinkFilters_missingURL(t *testing.T) {
	t.Parallel()
	handler := NewModeratorContentHandler(&fakeTrendsService{})

	b, _ := json.Marshal(map[string]string{"url": ""})
	req := httptest.NewRequest(http.MethodPost, "/moderator/content/trending-link-filters", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.POSTTrendingLinkFilters(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestModeratorContentHandler_DELETETrendingLinkFilter_valid(t *testing.T) {
	t.Parallel()
	svc := &fakeTrendsService{filters: []string{"https://spam.example/bad"}}
	handler := NewModeratorContentHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/moderator/content/trending-link-filters?url=https%3A%2F%2Fspam.example%2Fbad", nil)
	rec := httptest.NewRecorder()
	handler.DELETETrendingLinkFilter(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, svc.filters)
}

func TestModeratorContentHandler_DELETETrendingLinkFilter_missingURL(t *testing.T) {
	t.Parallel()
	handler := NewModeratorContentHandler(&fakeTrendsService{})

	req := httptest.NewRequest(http.MethodDelete, "/moderator/content/trending-link-filters", nil)
	rec := httptest.NewRecorder()
	handler.DELETETrendingLinkFilter(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
