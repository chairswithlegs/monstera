package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

// TrendsService provides trending statuses and tags from the pre-computed index.
type TrendsService interface {
	TrendingStatuses(ctx context.Context, limit int) ([]EnrichedStatus, error)
	TrendingTags(ctx context.Context, limit int) ([]domain.TrendingTag, error)
	RefreshIndexes(ctx context.Context) error
}

type trendingCache struct {
	statuses []EnrichedStatus
	tags     []domain.TrendingTag
}

type trendsService struct {
	store    store.Store
	mu       sync.RWMutex
	cached   *trendingCache
	cachedAt time.Time
	cacheTTL time.Duration
}

// NewTrendsService returns a TrendsService backed by the pre-computed trending index.
func NewTrendsService(s store.Store) TrendsService {
	return &trendsService{store: s, cacheTTL: 15 * time.Minute}
}

func (svc *trendsService) TrendingStatuses(ctx context.Context, limit int) ([]EnrichedStatus, error) {
	c, err := svc.getCache(ctx)
	if err != nil {
		return nil, err
	}
	out := c.statuses
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (svc *trendsService) TrendingTags(ctx context.Context, limit int) ([]domain.TrendingTag, error) {
	c, err := svc.getCache(ctx)
	if err != nil {
		return nil, err
	}
	out := c.tags
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// getCache returns a fresh cache, filling it when stale.
// Uses double-checked locking to avoid a thundering herd on cache expiry.
func (svc *trendsService) getCache(ctx context.Context) (*trendingCache, error) {
	svc.mu.RLock()
	if svc.cached != nil && time.Since(svc.cachedAt) < svc.cacheTTL {
		c := svc.cached
		svc.mu.RUnlock()
		return c, nil
	}
	svc.mu.RUnlock()

	svc.mu.Lock()
	defer svc.mu.Unlock()

	// Re-check after acquiring write lock.
	if svc.cached != nil && time.Since(svc.cachedAt) < svc.cacheTTL {
		return svc.cached, nil
	}

	c, err := svc.fill(ctx)
	if err != nil {
		return nil, err
	}
	svc.cached = c
	svc.cachedAt = time.Now()
	return c, nil
}

// fill fetches both trending statuses and tags from the store in one pass.
func (svc *trendsService) fill(ctx context.Context) (*trendingCache, error) {
	const maxTrending = 20

	trendingEntries, err := svc.store.GetTrendingStatusIDs(ctx, maxTrending)
	if err != nil {
		return nil, fmt.Errorf("GetTrendingStatusIDs: %w", err)
	}

	statuses := make([]EnrichedStatus, 0, len(trendingEntries))
	for _, entry := range trendingEntries {
		s, err := svc.store.GetStatusByID(ctx, entry.StatusID)
		if err != nil {
			slog.WarnContext(ctx, "trending status not found", slog.String("status_id", entry.StatusID))
			continue
		}
		author, err := svc.store.GetAccountByID(ctx, s.AccountID)
		if err != nil {
			slog.WarnContext(ctx, "trending status author not found",
				slog.String("status_id", s.ID), slog.String("account_id", s.AccountID))
			continue
		}
		mentions, err := svc.store.GetStatusMentions(ctx, s.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusMentions(%s): %w", s.ID, err)
		}
		tags, err := svc.store.GetStatusHashtags(ctx, s.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusHashtags(%s): %w", s.ID, err)
		}
		media, err := svc.store.GetStatusAttachments(ctx, s.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusAttachments(%s): %w", s.ID, err)
		}
		statuses = append(statuses, EnrichedStatus{
			Status:   s,
			Author:   author,
			Mentions: mentions,
			Tags:     tags,
			Media:    media,
		})
	}

	tags, err := svc.store.GetTrendingTags(ctx, 7, maxTrending)
	if err != nil {
		return nil, fmt.Errorf("GetTrendingTags: %w", err)
	}

	return &trendingCache{statuses: statuses, tags: tags}, nil
}

func (svc *trendsService) RefreshIndexes(ctx context.Context) error {
	// This is a super simple algorithm for getting "trending" statuses.
	// It simply gets the top 20 statuses by engagement score in the last 48 hours.
	// Engagement score is defined as the sum of reblogs, favourites and replies.
	scored, err := svc.store.GetTopScoredPublicStatuses(ctx, time.Now().UTC().Add(-48*time.Hour), 20)
	if err != nil {
		return fmt.Errorf("GetTopScoredPublicStatuses: %w", err)
	}
	entries := make([]store.TrendingStatusEntry, len(scored))
	for i, s := range scored {
		entries[i] = store.TrendingStatusEntry{StatusID: s.StatusID, Score: s.Score}
	}
	if err := svc.store.ReplaceTrendingStatuses(ctx, entries); err != nil {
		return fmt.Errorf("ReplaceTrendingStatuses: %w", err)
	}

	// This is a super simple algorithm for getting "trending" tags.
	// It tabulates the usage of each hashtags over the last 7 days.
	since := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -6)
	stats, err := svc.store.GetHashtagDailyStats(ctx, since)
	if err != nil {
		return fmt.Errorf("GetHashtagDailyStats: %w", err)
	}
	tagEntries := make([]store.TrendingTagHistoryEntry, len(stats))
	for i, s := range stats {
		tagEntries[i] = store.TrendingTagHistoryEntry{
			HashtagID: s.HashtagID,
			Day:       s.Day,
			Uses:      s.Uses,
			Accounts:  s.Accounts,
		}
	}
	if err := svc.store.UpsertTrendingTagHistory(ctx, tagEntries); err != nil {
		return fmt.Errorf("UpsertTrendingTagHistory: %w", err)
	}

	slog.InfoContext(ctx, "trending indexes updated",
		slog.Int("statuses", len(entries)),
		slog.Int("tag_days", len(tagEntries)))
	return nil
}
