package mastodon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// fakeTrendsService implements service.TrendsService for handler tests.
type fakeTrendsService struct {
	statuses []service.EnrichedStatus
	tags     []domain.TrendingTag
	err      error
}

func (f *fakeTrendsService) TrendingStatuses(_ context.Context, limit int) ([]service.EnrichedStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := f.statuses
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeTrendsService) TrendingTags(_ context.Context, limit int) ([]domain.TrendingTag, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := f.tags
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeTrendsService) RefreshIndexes(_ context.Context) error { return nil }

func TestTrendsHandler_GETTrendsStatuses_empty(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/statuses", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsStatuses(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}

func TestTrendsHandler_GETTrendsStatuses_withData(t *testing.T) {
	t.Parallel()
	content := "hello world"
	svc := &fakeTrendsService{
		statuses: []service.EnrichedStatus{
			{
				Status: &domain.Status{
					ID:         "01STATUSID",
					URI:        "https://example.com/statuses/01STATUSID",
					AccountID:  "01ACCOUNTID",
					Content:    &content,
					Visibility: "public",
					Local:      true,
					CreatedAt:  time.Now(),
				},
				Author: &domain.Account{
					ID:       "01ACCOUNTID",
					Username: "alice",
				},
				Mentions: nil,
				Tags:     nil,
				Media:    nil,
			},
		},
	}
	handler := NewTrendsHandler(svc, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/statuses", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsStatuses(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, "01STATUSID", body[0]["id"])
	assert.Equal(t, "alice", body[0]["account"].(map[string]any)["username"])
}

func TestTrendsHandler_GETTrendsStatuses_error(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{err: errors.New("store error")}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/statuses", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsStatuses(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTrendsHandler_GETTrendsTags_empty(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/tags?limit=20", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsTags(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}

func TestTrendsHandler_GETTrendsTags_withData(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := &fakeTrendsService{
		tags: []domain.TrendingTag{
			{
				Hashtag: domain.Hashtag{ID: "tag1", Name: "golang"},
				History: []domain.TagHistoryDay{
					{Day: day, Uses: 42, Accounts: 10},
				},
			},
		},
	}
	handler := NewTrendsHandler(svc, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/tags", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsTags(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Len(t, body, 1)
	assert.Equal(t, "golang", body[0]["name"])
	assert.Equal(t, "https://example.com/tags/golang", body[0]["url"])
	history, ok := body[0]["history"].([]any)
	require.True(t, ok)
	require.Len(t, history, 1)
	h := history[0].(map[string]any)
	assert.Equal(t, "42", h["uses"])
	assert.Equal(t, "10", h["accounts"])
}

func TestTrendsHandler_GETTrendsTags_error(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{err: errors.New("store error")}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/tags", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsTags(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTrendsHandler_GETTrendsLinks(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/links", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsLinks(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}
