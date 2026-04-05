package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chairswithlegs/monstera/internal/domain"
	db "github.com/chairswithlegs/monstera/internal/store/postgres/generated"
)

// GetTopScoredPublicStatuses returns up to limit public statuses created since `since`,
// ordered by a simple engagement score (reblogs + favourites + 0.5×replies).
// When localOnly is true only statuses from local accounts are included.
func (s *PostgresStore) GetTopScoredPublicStatuses(ctx context.Context, since time.Time, limit int, localOnly bool) ([]domain.TrendingStatus, error) {
	rows, err := s.q.GetTopScoredPublicStatuses(ctx, db.GetTopScoredPublicStatusesParams{
		Since:      pgtype.Timestamptz{Time: since, Valid: true},
		LocalOnly:  localOnly,
		MaxResults: int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, fmt.Errorf("GetTopScoredPublicStatuses: %w", mapErr(err))
	}

	out := make([]domain.TrendingStatus, len(rows))
	for i, r := range rows {
		out[i] = domain.TrendingStatus{
			StatusID: r.StatusID,
			Score:    r.Score,
		}
	}
	return out, nil
}

// GetHashtagDailyStats returns per-hashtag per-day usage aggregates since `since`.
// When localOnly is true only statuses from local accounts are included.
func (s *PostgresStore) GetHashtagDailyStats(ctx context.Context, since time.Time, localOnly bool) ([]domain.HashtagDailyStats, error) {
	rows, err := s.q.GetHashtagDailyStats(ctx, db.GetHashtagDailyStatsParams{
		Since:     pgtype.Timestamptz{Time: since, Valid: true},
		LocalOnly: localOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("GetHashtagDailyStats: %w", mapErr(err))
	}

	out := make([]domain.HashtagDailyStats, len(rows))
	for i, r := range rows {
		out[i] = domain.HashtagDailyStats{
			HashtagID:   r.HashtagID,
			HashtagName: r.HashtagName,
			Uses:        r.Uses,
			Accounts:    r.Accounts,
		}
		if r.Day.Valid {
			out[i].Day = r.Day.Time
		}
	}
	return out, nil
}

// ReplaceTrendingStatuses replaces the entire trending_statuses index atomically.
func (s *PostgresStore) ReplaceTrendingStatuses(ctx context.Context, entries []domain.TrendingStatus) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ReplaceTrendingStatuses begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	if err := qtx.TruncateTrendingStatuses(ctx); err != nil {
		return fmt.Errorf("ReplaceTrendingStatuses truncate: %w", mapErr(err))
	}

	if len(entries) > 0 {
		ids := make([]string, len(entries))
		scores := make([]float64, len(entries))
		for i, e := range entries {
			ids[i] = e.StatusID
			scores[i] = e.Score
		}

		if err := qtx.BulkUpsertTrendingStatuses(ctx, db.BulkUpsertTrendingStatusesParams{
			Column1: ids,
			Column2: scores,
		}); err != nil {
			return fmt.Errorf("ReplaceTrendingStatuses upsert: %w", mapErr(err))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ReplaceTrendingStatuses commit: %w", err)
	}
	return nil
}

// GetTrendingStatusIDs returns up to limit entries from the trending_statuses index.
func (s *PostgresStore) GetTrendingStatusIDs(ctx context.Context, limit int) ([]domain.TrendingStatus, error) {
	rows, err := s.q.GetTrendingStatuses(ctx, int32(limit)) //nolint:gosec // limit clamped by caller
	if err != nil {
		return nil, fmt.Errorf("GetTrendingStatusIDs: %w", mapErr(err))
	}

	out := make([]domain.TrendingStatus, len(rows))
	for i, r := range rows {
		out[i] = domain.TrendingStatus{
			StatusID: r.StatusID,
			Score:    r.Score,
			RankedAt: pgTime(r.RankedAt),
		}
	}
	return out, nil
}

// TruncateTrendingTagHistory removes all rows from trending_tag_history.
func (s *PostgresStore) TruncateTrendingTagHistory(ctx context.Context) error {
	if err := s.q.TruncateTrendingTagHistory(ctx); err != nil {
		return fmt.Errorf("TruncateTrendingTagHistory: %w", mapErr(err))
	}
	return nil
}

// UpsertTrendingTagHistory inserts or updates daily usage entries in trending_tag_history.
func (s *PostgresStore) UpsertTrendingTagHistory(ctx context.Context, entries []domain.TrendingTagHistory) error {
	if len(entries) == 0 {
		return nil
	}

	ids := make([]string, len(entries))
	days := make([]pgtype.Date, len(entries))
	uses := make([]int64, len(entries))
	accounts := make([]int64, len(entries))
	for i, e := range entries {
		ids[i] = e.HashtagID
		days[i] = pgtype.Date{Time: e.Day, Valid: true}
		uses[i] = e.Uses
		accounts[i] = e.Accounts
	}

	if err := s.q.UpsertTrendingTagHistory(ctx, db.UpsertTrendingTagHistoryParams{
		Column1: ids,
		Column2: days,
		Column3: uses,
		Column4: accounts,
	}); err != nil {
		return fmt.Errorf("UpsertTrendingTagHistory: %w", mapErr(err))
	}
	return nil
}

// GetLinkDailyStats returns per-URL per-day usage aggregates for the past `days` days,
// computed fresh from status_cards JOIN statuses. When localOnly is true only statuses
// from local accounts are included.
func (s *PostgresStore) GetLinkDailyStats(ctx context.Context, days int, localOnly bool) ([]domain.TrendingLinkStats, error) {
	rows, err := s.q.GetLinkDailyStats(ctx, db.GetLinkDailyStatsParams{
		Column1: int32(days), //nolint:gosec // days is a small constant
		Column2: localOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("GetLinkDailyStats: %w", mapErr(err))
	}

	out := make([]domain.TrendingLinkStats, len(rows))
	for i, r := range rows {
		out[i] = domain.TrendingLinkStats{
			URL:      r.Url,
			Uses:     r.Uses,
			Accounts: r.Accounts,
		}
		if r.Day.Valid {
			out[i].Day = r.Day.Time
		}
	}
	return out, nil
}

// UpsertTrendingLinkHistory inserts or updates daily usage entries in trending_link_history.
func (s *PostgresStore) UpsertTrendingLinkHistory(ctx context.Context, entries []domain.TrendingLinkStats) error {
	if len(entries) == 0 {
		return nil
	}

	urls := make([]string, len(entries))
	days := make([]pgtype.Date, len(entries))
	uses := make([]int64, len(entries))
	accounts := make([]int64, len(entries))
	for i, e := range entries {
		urls[i] = e.URL
		days[i] = pgtype.Date{Time: e.Day, Valid: true}
		uses[i] = e.Uses
		accounts[i] = e.Accounts
	}

	if err := s.q.UpsertTrendingLinkHistory(ctx, db.UpsertTrendingLinkHistoryParams{
		Column1: urls,
		Column2: days,
		Column3: uses,
		Column4: accounts,
	}); err != nil {
		return fmt.Errorf("UpsertTrendingLinkHistory: %w", mapErr(err))
	}
	return nil
}

// ReplaceTrendingLinks replaces the entire trending_links index in a single transaction.
func (s *PostgresStore) ReplaceTrendingLinks(ctx context.Context, entries []domain.TrendingLink) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ReplaceTrendingLinks begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	if err := qtx.ReplaceTrendingLinks(ctx); err != nil {
		return fmt.Errorf("ReplaceTrendingLinks truncate: %w", mapErr(err))
	}

	if len(entries) > 0 {
		urls := make([]string, len(entries))
		scores := make([]float64, len(entries))
		for i, e := range entries {
			urls[i] = e.URL
			// Compute score as total uses across history days.
			var score float64
			for _, h := range e.History {
				score += float64(h.Uses)
			}
			scores[i] = score
		}

		if err := qtx.BulkInsertTrendingLinks(ctx, db.BulkInsertTrendingLinksParams{
			Column1: urls,
			Column2: scores,
		}); err != nil {
			return fmt.Errorf("ReplaceTrendingLinks insert: %w", mapErr(err))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ReplaceTrendingLinks commit: %w", err)
	}
	return nil
}

// GetTrendingLinks returns trending links with up to `days` days of history,
// limited to `limit` links ordered by score descending. Card metadata is
// enriched from the most recently fetched status_cards row for each URL.
func (s *PostgresStore) GetTrendingLinks(ctx context.Context, days int, limit int) ([]domain.TrendingLink, error) {
	rows, err := s.q.GetTrendingLinks(ctx, db.GetTrendingLinksParams{
		Column1: int32(days),  //nolint:gosec // days is a small constant
		Limit:   int32(limit), //nolint:gosec // limit clamped by caller
	})
	if err != nil {
		return nil, fmt.Errorf("GetTrendingLinks: %w", mapErr(err))
	}

	type linkEntry struct {
		link    domain.TrendingLink
		history []domain.TrendingLinkHistoryDay
	}
	order := make([]string, 0)
	byURL := make(map[string]*linkEntry)

	for _, r := range rows {
		e, ok := byURL[r.Url]
		if !ok {
			e = &linkEntry{link: domain.TrendingLink{
				URL:          r.Url,
				Title:        r.Title,
				Description:  r.Description,
				Type:         r.CardType,
				ProviderName: r.ProviderName,
				ProviderURL:  r.ProviderUrl,
				ImageURL:     r.ImageUrl,
				Width:        int(r.Width),
				Height:       int(r.Height),
			}}
			byURL[r.Url] = e
			order = append(order, r.Url)
		}
		if r.Uses != nil && r.Accounts != nil && r.Day.Valid {
			e.history = append(e.history, domain.TrendingLinkHistoryDay{
				Day:      r.Day.Time,
				Uses:     *r.Uses,
				Accounts: *r.Accounts,
			})
		}
	}

	out := make([]domain.TrendingLink, 0, len(order))
	for _, url := range order {
		e := byURL[url]
		e.link.History = e.history
		out = append(out, e.link)
	}
	return out, nil
}

// GetTrendingTags returns trending hashtags with up to `days` days of history,
// limited to `limit` hashtags ordered by total recent uses (descending).
func (s *PostgresStore) GetTrendingTags(ctx context.Context, days int, limit int) ([]domain.TrendingTag, error) {
	rows, err := s.q.GetTrendingTagHistory(ctx, int32(days)) //nolint:gosec // days is a small constant
	if err != nil {
		return nil, fmt.Errorf("GetTrendingTags: %w", mapErr(err))
	}

	type tagKey = string
	type tagEntry struct {
		hashtag   domain.Hashtag
		totalUses int64
		history   []domain.TagHistoryDay
	}
	order := make([]tagKey, 0)
	byID := make(map[tagKey]*tagEntry)

	for _, r := range rows {
		e, ok := byID[r.ID]
		if !ok {
			e = &tagEntry{
				hashtag: domain.Hashtag{
					ID:        r.ID,
					Name:      r.Name,
					CreatedAt: pgTime(r.CreatedAt),
					UpdatedAt: pgTime(r.UpdatedAt),
				},
			}
			byID[r.ID] = e
			order = append(order, r.ID)
		}
		var dayTime time.Time
		if r.Day.Valid {
			dayTime = r.Day.Time
		}
		e.totalUses += r.Uses
		e.history = append(e.history, domain.TagHistoryDay{
			Day:      dayTime,
			Uses:     r.Uses,
			Accounts: r.Accounts,
		})
	}

	// Sort by total uses descending and cap at limit.
	entries := make([]*tagEntry, 0, len(order))
	for _, id := range order {
		entries = append(entries, byID[id])
	}
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].totalUses > entries[j-1].totalUses; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}

	out := make([]domain.TrendingTag, len(entries))
	for i, e := range entries {
		out[i] = domain.TrendingTag{
			Hashtag: e.hashtag,
			History: e.history,
		}
	}
	return out, nil
}
