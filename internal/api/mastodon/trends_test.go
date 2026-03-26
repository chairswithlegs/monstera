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

	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// fakeTrendsService implements service.TrendsService for handler tests.
type fakeTrendsService struct {
	statuses []service.EnrichedStatus
	tags     []domain.TrendingTag
	err      error
}

func (f *fakeTrendsService) TrendingStatuses(_ context.Context, offset, limit int) ([]service.EnrichedStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := f.statuses
	if offset >= len(out) {
		return []service.EnrichedStatus{}, nil
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeTrendsService) TrendingTags(_ context.Context, offset, limit int) ([]domain.TrendingTag, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := f.tags
	if offset >= len(out) {
		return []domain.TrendingTag{}, nil
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeTrendsService) RefreshIndexes(_ context.Context) error { return nil }

// fakeTagFollowService is a minimal TagFollowService for trends handler tests.
type fakeTagFollowService struct {
	followed []domain.Hashtag
	err      error
}

func (f *fakeTagFollowService) GetTagByName(_ context.Context, _ string) (*domain.Hashtag, error) {
	return nil, nil
}
func (f *fakeTagFollowService) IsFollowingTag(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}
func (f *fakeTagFollowService) ListFollowedTags(_ context.Context, _ string, _ *string, _ int) ([]domain.Hashtag, *string, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.followed, nil, nil
}
func (f *fakeTagFollowService) FollowTag(_ context.Context, _, _ string) (*domain.Hashtag, error) {
	return nil, nil
}
func (f *fakeTagFollowService) UnfollowTag(_ context.Context, _, _ string) error { return nil }
func (f *fakeTagFollowService) UnfollowTagByName(_ context.Context, _, _ string) (*domain.Hashtag, error) {
	return nil, nil
}
func (f *fakeTagFollowService) AreFollowingTagsByName(_ context.Context, _ string, tagNames []string) (map[string]bool, error) {
	if f.err != nil {
		return nil, f.err
	}
	followedSet := make(map[string]bool, len(f.followed))
	for _, h := range f.followed {
		followedSet[h.Name] = true
	}
	out := make(map[string]bool, len(tagNames))
	for _, n := range tagNames {
		out[n] = followedSet[n]
	}
	return out, nil
}

func TestTrendsHandler_GETTrendsStatuses_empty(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{}, &fakeTagFollowService{}, "example.com")

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
	handler := NewTrendsHandler(svc, &fakeTagFollowService{}, "example.com")

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
	handler := NewTrendsHandler(&fakeTrendsService{err: errors.New("store error")}, &fakeTagFollowService{}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/statuses", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsStatuses(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTrendsHandler_GETTrendsStatuses_offset(t *testing.T) {
	t.Parallel()
	makeStatus := func(id string) service.EnrichedStatus {
		content := "hello " + id
		return service.EnrichedStatus{
			Status: &domain.Status{
				ID:         id,
				URI:        "https://example.com/statuses/" + id,
				AccountID:  "01ACCOUNTID",
				Content:    &content,
				Visibility: "public",
				Local:      true,
				CreatedAt:  time.Now(),
			},
			Author: &domain.Account{ID: "01ACCOUNTID", Username: "alice"},
		}
	}
	svc := &fakeTrendsService{
		statuses: []service.EnrichedStatus{makeStatus("s1"), makeStatus("s2"), makeStatus("s3")},
	}
	handler := NewTrendsHandler(svc, &fakeTagFollowService{}, "example.com")

	cases := []struct {
		query   string
		wantIDs []string
	}{
		{"?offset=0&limit=2", []string{"s1", "s2"}},
		{"?offset=2&limit=2", []string{"s3"}},
		{"?offset=10&limit=2", []string{}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/statuses"+tc.query, nil)
		rec := httptest.NewRecorder()
		handler.GETTrendsStatuses(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		ids := make([]string, len(body))
		for i, s := range body {
			ids[i] = s["id"].(string)
		}
		assert.Equal(t, tc.wantIDs, ids, "query=%s", tc.query)
	}
}

func TestTrendsHandler_GETTrendsTags_empty(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{}, &fakeTagFollowService{}, "example.com")

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
	handler := NewTrendsHandler(svc, &fakeTagFollowService{}, "example.com")

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

func TestTrendsHandler_GETTrendsTags_offset(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := &fakeTrendsService{
		tags: []domain.TrendingTag{
			{Hashtag: domain.Hashtag{ID: "tag1", Name: "golang"}, History: []domain.TagHistoryDay{{Day: day, Uses: 10, Accounts: 5}}},
			{Hashtag: domain.Hashtag{ID: "tag2", Name: "rust"}, History: []domain.TagHistoryDay{{Day: day, Uses: 8, Accounts: 3}}},
			{Hashtag: domain.Hashtag{ID: "tag3", Name: "zig"}, History: []domain.TagHistoryDay{{Day: day, Uses: 4, Accounts: 2}}},
		},
	}
	handler := NewTrendsHandler(svc, &fakeTagFollowService{}, "example.com")

	cases := []struct {
		query     string
		wantNames []string
	}{
		{"?offset=1&limit=1", []string{"rust"}},
		{"?offset=2&limit=5", []string{"zig"}},
		{"?offset=5&limit=5", []string{}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/tags"+tc.query, nil)
		rec := httptest.NewRecorder()
		handler.GETTrendsTags(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var body []map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		names := make([]string, len(body))
		for i, tag := range body {
			names[i] = tag["name"].(string)
		}
		assert.Equal(t, tc.wantNames, names, "query=%s", tc.query)
	}
}

func TestTrendsHandler_GETTrendsTags_error(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{err: errors.New("store error")}, &fakeTagFollowService{}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/tags", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsTags(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTrendsHandler_GETTrendsLinks(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler(&fakeTrendsService{}, &fakeTagFollowService{}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/links", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsLinks(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}

func TestTrendsHandler_GETTrendsTags_followingOmittedWhenUnauthenticated(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := &fakeTrendsService{
		tags: []domain.TrendingTag{
			{Hashtag: domain.Hashtag{ID: "tag1", Name: "golang"}, History: []domain.TagHistoryDay{{Day: day, Uses: 5, Accounts: 2}}},
		},
	}
	handler := NewTrendsHandler(svc, &fakeTagFollowService{}, "example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/tags", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsTags(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Len(t, body, 1)
	_, hasFollowing := body[0]["following"]
	assert.False(t, hasFollowing, "following should be absent for unauthenticated requests")
}

func TestTrendsHandler_GETTrendsTags_followingReflectsUserState(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := &fakeTrendsService{
		tags: []domain.TrendingTag{
			{Hashtag: domain.Hashtag{ID: "tag1", Name: "golang"}, History: []domain.TagHistoryDay{{Day: day, Uses: 5, Accounts: 2}}},
			{Hashtag: domain.Hashtag{ID: "tag2", Name: "rust"}, History: []domain.TagHistoryDay{{Day: day, Uses: 3, Accounts: 1}}},
		},
	}
	tagFollows := &fakeTagFollowService{
		followed: []domain.Hashtag{{ID: "tag1", Name: "golang"}},
	}
	handler := NewTrendsHandler(svc, tagFollows, "example.com")

	account := &domain.Account{ID: "acct1", Username: "alice"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/tags", nil)
	req = req.WithContext(middleware.WithAccount(req.Context(), account))
	rec := httptest.NewRecorder()
	handler.GETTrendsTags(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Len(t, body, 2)

	byName := make(map[string]map[string]any, len(body))
	for _, tag := range body {
		byName[tag["name"].(string)] = tag
	}
	assert.Equal(t, true, byName["golang"]["following"], "golang is followed")
	assert.Equal(t, false, byName["rust"]["following"], "rust is not followed")
}
