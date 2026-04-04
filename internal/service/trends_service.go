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

// TrendsService provides trending statuses, tags, and links from the pre-computed index.
type TrendsService interface {
	TrendingStatuses(ctx context.Context, offset, limit int) ([]EnrichedStatus, error)
	TrendingTags(ctx context.Context, offset, limit int) ([]domain.TrendingTag, error)
	TrendingLinks(ctx context.Context, offset, limit int) ([]domain.TrendingLink, error)
	RefreshIndexes(ctx context.Context) error
}

type trendingCache struct {
	statuses []EnrichedStatus
	tags     []domain.TrendingTag
	links    []domain.TrendingLink
}

type trendsService struct {
	store     store.Store
	statusSvc StatusService
	mu        sync.RWMutex
	cached    *trendingCache
	cachedAt  time.Time
	cacheTTL  time.Duration
}

// NewTrendsService returns a TrendsService backed by the pre-computed trending index.
func NewTrendsService(s store.Store, statusSvc StatusService) TrendsService {
	return &trendsService{store: s, statusSvc: statusSvc, cacheTTL: 15 * time.Minute}
}

func (svc *trendsService) TrendingStatuses(ctx context.Context, offset, limit int) ([]EnrichedStatus, error) {
	c, err := svc.getCache(ctx)
	if err != nil {
		return nil, err
	}
	out := c.statuses
	if offset >= len(out) {
		return []EnrichedStatus{}, nil
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (svc *trendsService) TrendingTags(ctx context.Context, offset, limit int) ([]domain.TrendingTag, error) {
	c, err := svc.getCache(ctx)
	if err != nil {
		return nil, err
	}
	out := c.tags
	if offset >= len(out) {
		return []domain.TrendingTag{}, nil
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (svc *trendsService) TrendingLinks(ctx context.Context, offset, limit int) ([]domain.TrendingLink, error) {
	c, err := svc.getCache(ctx)
	if err != nil {
		return nil, err
	}
	out := c.links
	if offset >= len(out) {
		return []domain.TrendingLink{}, nil
	}
	out = out[offset:]
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

	statuses := make([]*domain.Status, 0, len(trendingEntries))
	for _, entry := range trendingEntries {
		s, err := svc.store.GetStatusByID(ctx, entry.StatusID)
		if err != nil {
			slog.WarnContext(ctx, "trending status not found", slog.String("status_id", entry.StatusID))
			continue
		}
		statuses = append(statuses, s)
	}
	enriched, err := svc.statusSvc.EnrichStatuses(ctx, statuses, EnrichOpts{})
	if err != nil {
		return nil, fmt.Errorf("EnrichStatuses: %w", err)
	}

	tags, err := svc.store.GetTrendingTags(ctx, 7, maxTrending)
	if err != nil {
		return nil, fmt.Errorf("GetTrendingTags: %w", err)
	}

	links, err := svc.store.GetTrendingLinks(ctx, 7, maxTrending)
	if err != nil {
		return nil, fmt.Errorf("GetTrendingLinks: %w", err)
	}

	return &trendingCache{statuses: enriched, tags: tags, links: links}, nil
}

func (svc *trendsService) RefreshIndexes(ctx context.Context) error {
	settings, err := svc.store.GetMonsteraSettings(ctx)
	if err != nil {
		return fmt.Errorf("GetMonsteraSettings: %w", err)
	}
	var statusesScope, tagsScope, linksScope domain.MonsteraTrendingScope
	if settings != nil {
		statusesScope = settings.TrendingStatusesScope
		tagsScope = settings.TrendingTagsScope
		linksScope = settings.TrendingLinksScope
	}

	// Trending statuses: scope controls which statuses are sourced.
	// This is a super simple algorithm for getting "trending" statuses.
	// It simply gets the top 20 statuses by engagement score in the last 48 hours.
	// Engagement score is defined as the sum of reblogs, favourites and replies.
	if statusesScope == domain.MonsteraTrendingDisabled || statusesScope == "" {
		if err := svc.store.ReplaceTrendingStatuses(ctx, nil); err != nil {
			return fmt.Errorf("ReplaceTrendingStatuses (clear): %w", err)
		}
	} else {
		localOnly := statusesScope == domain.MonsteraTrendingLocal
		scored, err := svc.store.GetTopScoredPublicStatuses(ctx, time.Now().UTC().Add(-48*time.Hour), 20, localOnly)
		if err != nil {
			return fmt.Errorf("GetTopScoredPublicStatuses: %w", err)
		}
		if err := svc.store.ReplaceTrendingStatuses(ctx, scored); err != nil {
			return fmt.Errorf("ReplaceTrendingStatuses: %w", err)
		}
	}

	// Trending tags: scope controls which statuses are sourced.
	// It tabulates the usage of each hashtag over the last 7 days.
	since := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -6)
	if tagsScope == domain.MonsteraTrendingDisabled || tagsScope == "" {
		if err := svc.store.TruncateTrendingTagHistory(ctx); err != nil {
			return fmt.Errorf("TruncateTrendingTagHistory: %w", err)
		}
	} else {
		localOnly := tagsScope == domain.MonsteraTrendingLocal
		stats, err := svc.store.GetHashtagDailyStats(ctx, since, localOnly)
		if err != nil {
			return fmt.Errorf("GetHashtagDailyStats: %w", err)
		}
		tagEntries := make([]domain.TrendingTagHistory, len(stats))
		for i, s := range stats {
			tagEntries[i] = domain.TrendingTagHistory{
				HashtagID: s.HashtagID,
				Day:       s.Day,
				Uses:      s.Uses,
				Accounts:  s.Accounts,
			}
		}
		if err := svc.store.UpsertTrendingTagHistory(ctx, tagEntries); err != nil {
			return fmt.Errorf("UpsertTrendingTagHistory: %w", err)
		}
	}

	// Trending links: scope controls which statuses are sourced; disabled clears the index.
	var linkStats []domain.TrendingLinkStats
	if linksScope == domain.MonsteraTrendingDisabled || linksScope == "" {
		// Disabled: clear the index and skip link indexing.
		if err := svc.store.ReplaceTrendingLinks(ctx, nil); err != nil {
			return fmt.Errorf("ReplaceTrendingLinks (clear): %w", err)
		}
		slog.InfoContext(ctx, "trending indexes updated", slog.Int("link_days", 0))
		return nil
	}

	// local: index links from local statuses only.
	// all: index links from all statuses (local + remote).
	// Note: "all" currently behaves like "local" because card_subscriber.go only
	// fetches cards for local statuses. Remote card processing is a follow-up.
	localOnly := linksScope == domain.MonsteraTrendingLocal
	var fetchErr error
	linkStats, fetchErr = svc.store.GetLinkDailyStats(ctx, 7, localOnly)
	if fetchErr != nil {
		return fmt.Errorf("GetLinkDailyStats: %w", fetchErr)
	}
	if err := svc.store.UpsertTrendingLinkHistory(ctx, linkStats); err != nil {
		return fmt.Errorf("UpsertTrendingLinkHistory: %w", err)
	}
	// Build TrendingLink entries from daily stats.
	type linkEntry struct {
		totalUses int64
		history   []domain.TrendingLinkHistoryDay
	}
	order := make([]string, 0)
	byURL := make(map[string]*linkEntry)
	for _, s := range linkStats {
		e, ok := byURL[s.URL]
		if !ok {
			e = &linkEntry{}
			byURL[s.URL] = e
			order = append(order, s.URL)
		}
		e.totalUses += s.Uses
		e.history = append(e.history, domain.TrendingLinkHistoryDay{Day: s.Day, Uses: s.Uses, Accounts: s.Accounts})
	}
	denied, err := svc.store.ListTrendingLinkDenylist(ctx)
	if err != nil {
		return fmt.Errorf("ListTrendingLinkDenylist: %w", err)
	}
	deniedSet := make(map[string]struct{}, len(denied))
	for _, u := range denied {
		deniedSet[u] = struct{}{}
	}

	linkEntries := make([]domain.TrendingLink, 0, len(order))
	for _, url := range order {
		if _, blocked := deniedSet[url]; blocked {
			continue
		}
		e := byURL[url]
		linkEntries = append(linkEntries, domain.TrendingLink{URL: url, History: e.history})
	}
	if err := svc.store.ReplaceTrendingLinks(ctx, linkEntries); err != nil {
		return fmt.Errorf("ReplaceTrendingLinks: %w", err)
	}

	slog.InfoContext(ctx, "trending indexes updated",
		slog.Int("link_days", len(linkStats)))
	return nil
}
