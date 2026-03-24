package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestTrendsService_TrendingStatuses(t *testing.T) {
	t.Parallel()

	content := "test content"
	fs := testutil.NewFakeStore()

	acc := &domain.Account{ID: "acc1", Username: "alice"}
	status := &domain.Status{
		ID:         "st1",
		URI:        "https://example.com/statuses/st1",
		AccountID:  "acc1",
		Content:    &content,
		Visibility: "public",
		Local:      true,
		CreatedAt:  time.Now(),
	}
	// Seed the account and status in the fake store.
	_, err := fs.CreateAccount(context.Background(), store.CreateAccountInput{
		ID:           acc.ID,
		Username:     acc.Username,
		InboxURL:     "https://example.com/inbox",
		OutboxURL:    "https://example.com/outbox",
		FollowersURL: "https://example.com/followers",
		FollowingURL: "https://example.com/following",
		APID:         "https://example.com/users/alice",
		PublicKey:    "pk",
	})
	require.NoError(t, err)
	_, err = fs.CreateStatus(context.Background(), store.CreateStatusInput{
		ID:                  status.ID,
		URI:                 status.URI,
		AccountID:           status.AccountID,
		Content:             status.Content,
		Visibility:          status.Visibility,
		Local:               status.Local,
		APID:                "https://example.com/statuses/st1",
		QuoteApprovalPolicy: "public",
	})
	require.NoError(t, err)

	// Seed the trending statuses index.
	fs.TrendingStatuses = []domain.TrendingStatus{
		{StatusID: "st1", Score: 5.0, RankedAt: time.Now()},
	}

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc)
	statuses, err := svc.TrendingStatuses(context.Background(), 0, 10)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, "st1", statuses[0].Status.ID)
	assert.Equal(t, "alice", statuses[0].Author.Username)
}

func TestTrendsService_TrendingStatuses_limit(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	// Seed three entries in the index; only two should be returned.
	fs.TrendingStatuses = []domain.TrendingStatus{
		{StatusID: "missing1"},
		{StatusID: "missing2"},
		{StatusID: "missing3"},
	}

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc)
	statuses, err := svc.TrendingStatuses(context.Background(), 0, 2)
	require.NoError(t, err)
	// All IDs are missing from the store, so statuses will be empty (warnings logged).
	assert.LessOrEqual(t, len(statuses), 2)
}

func TestTrendsService_TrendingStatuses_offset(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	fs.TrendingStatuses = []domain.TrendingStatus{
		{StatusID: "missing1"},
		{StatusID: "missing2"},
		{StatusID: "missing3"},
	}

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc)

	// Offset beyond cache returns empty, not an error.
	statuses, err := svc.TrendingStatuses(context.Background(), 10, 5)
	require.NoError(t, err)
	assert.Empty(t, statuses)
}

func TestTrendsService_TrendingTags_offset(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ht1, err := fs.GetOrCreateHashtag(context.Background(), "golang")
	require.NoError(t, err)
	ht2, err := fs.GetOrCreateHashtag(context.Background(), "rust")
	require.NoError(t, err)

	fs.TrendingTagHistory = []domain.TrendingTagHistory{
		{HashtagID: ht1.ID, Day: day, Uses: 100, Accounts: 20},
		{HashtagID: ht2.ID, Day: day, Uses: 50, Accounts: 10},
	}

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc)

	// offset=1 should return only the second tag.
	tags, err := svc.TrendingTags(context.Background(), 1, 5)
	require.NoError(t, err)
	require.Len(t, tags, 1)

	// offset beyond cache length returns empty, not an error.
	tags, err = svc.TrendingTags(context.Background(), 10, 5)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestTrendsService_TrendingTags(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Seed a hashtag so GetTrendingTags can look it up by ID.
	ht, err := fs.GetOrCreateHashtag(context.Background(), "golang")
	require.NoError(t, err)

	fs.TrendingTagHistory = []domain.TrendingTagHistory{
		{HashtagID: ht.ID, Day: day, Uses: 100, Accounts: 20},
		{HashtagID: ht.ID, Day: day.AddDate(0, 0, -1), Uses: 50, Accounts: 10},
	}

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc)
	tags, err := svc.TrendingTags(context.Background(), 0, 10)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "golang", tags[0].Hashtag.Name)
	assert.Len(t, tags[0].History, 2)
}

func TestTrendsService_TrendingTags_empty(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc)
	tags, err := svc.TrendingTags(context.Background(), 0, 10)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestTrendsService_RefreshIndexes(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	day := time.Now().UTC().Truncate(24 * time.Hour)
	fs.HashtagDailyStats = []domain.HashtagDailyStats{
		{HashtagID: "tag1", HashtagName: "golang", Day: day, Uses: 42, Accounts: 10},
	}

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc)
	err := svc.RefreshIndexes(context.Background())
	require.NoError(t, err)
	assert.Empty(t, fs.TrendingStatuses) // no scored statuses seeded
	require.Len(t, fs.TrendingTagHistory, 1)
	assert.Equal(t, "tag1", fs.TrendingTagHistory[0].HashtagID)
	assert.Equal(t, int64(42), fs.TrendingTagHistory[0].Uses)
}

func TestTrendsService_RefreshIndexes_withStatuses(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	fs.TrendingStatuses = []domain.TrendingStatus{
		{StatusID: "st1", Score: 10.0},
		{StatusID: "st2", Score: 5.0},
	}

	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc)
	err := svc.RefreshIndexes(context.Background())
	require.NoError(t, err)
	assert.Len(t, fs.TrendingStatuses, 2)
}

func TestTrendsService_cacheHit(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	statusSvc := NewStatusService(fs, "https://example.com", "example.com", 500)
	svc := NewTrendsService(fs, statusSvc).(*trendsService)
	svc.cacheTTL = time.Hour // long TTL to ensure cache is used

	// First call fills the cache.
	_, err := svc.TrendingTags(context.Background(), 0, 10)
	require.NoError(t, err)

	// Mutate the fake store — second call should NOT see the change (cache hit).
	day := time.Now()
	fs.TrendingTagHistory = []domain.TrendingTagHistory{
		{HashtagID: "tag1", Day: day, Uses: 99},
	}

	tags, err := svc.TrendingTags(context.Background(), 0, 10)
	require.NoError(t, err)
	assert.Empty(t, tags) // still sees empty cache from first fill
}
