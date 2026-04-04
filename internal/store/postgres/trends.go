package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// GetTopScoredPublicStatuses returns up to limit public statuses created since `since`,
// ordered by a simple engagement score (reblogs + favourites + 0.5×replies).
func (s *PostgresStore) GetTopScoredPublicStatuses(ctx context.Context, since time.Time, limit int) ([]domain.TrendingStatus, error) {
	const q = `
		SELECT id AS status_id,
		       (reblogs_count + favourites_count + replies_count * 0.5) AS score
		FROM statuses
		WHERE deleted_at IS NULL
		  AND visibility = 'public'
		  AND reblog_of_id IS NULL
		  AND created_at >= $1
		ORDER BY score DESC
		LIMIT $2`

	rows, err := s.pool.Query(ctx, q, pgtype.Timestamptz{Time: since, Valid: true}, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("GetTopScoredPublicStatuses: %w", mapErr(err))
	}
	defer rows.Close()

	var out []domain.TrendingStatus
	for rows.Next() {
		var ts domain.TrendingStatus
		if err := rows.Scan(&ts.StatusID, &ts.Score); err != nil {
			return nil, fmt.Errorf("GetTopScoredPublicStatuses scan: %w", mapErr(err))
		}
		out = append(out, ts)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetTopScoredPublicStatuses rows: %w", mapErr(err))
	}
	return out, nil
}

// GetHashtagDailyStats returns per-hashtag per-day usage aggregates since `since`.
func (s *PostgresStore) GetHashtagDailyStats(ctx context.Context, since time.Time) ([]domain.HashtagDailyStats, error) {
	const q = `
		SELECT h.id AS hashtag_id, h.name AS hashtag_name,
		       date_trunc('day', s.created_at AT TIME ZONE 'UTC')::date AS day,
		       COUNT(*) AS uses,
		       COUNT(DISTINCT s.account_id) AS accounts
		FROM status_hashtags sh
		JOIN statuses  s ON s.id  = sh.status_id
		JOIN hashtags  h ON h.id  = sh.hashtag_id
		WHERE s.deleted_at IS NULL
		  AND s.visibility IN ('public', 'unlisted')
		  AND s.created_at >= $1
		GROUP BY h.id, h.name, day
		ORDER BY day DESC, uses DESC`

	rows, err := s.pool.Query(ctx, q, pgtype.Timestamptz{Time: since, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("GetHashtagDailyStats: %w", mapErr(err))
	}
	defer rows.Close()

	var out []domain.HashtagDailyStats
	for rows.Next() {
		var hs domain.HashtagDailyStats
		var day pgtype.Date
		if err := rows.Scan(&hs.HashtagID, &hs.HashtagName, &day, &hs.Uses, &hs.Accounts); err != nil {
			return nil, fmt.Errorf("GetHashtagDailyStats scan: %w", mapErr(err))
		}
		if day.Valid {
			hs.Day = day.Time
		}
		out = append(out, hs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetHashtagDailyStats rows: %w", mapErr(err))
	}
	return out, nil
}

// ReplaceTrendingStatuses replaces the entire trending_statuses index atomically.
func (s *PostgresStore) ReplaceTrendingStatuses(ctx context.Context, entries []domain.TrendingStatus) error {
	if len(entries) == 0 {
		if _, err := s.pool.Exec(ctx, `DELETE FROM trending_statuses`); err != nil {
			return fmt.Errorf("ReplaceTrendingStatuses truncate: %w", mapErr(err))
		}
		return nil
	}

	ids := make([]string, len(entries))
	scores := make([]float64, len(entries))
	for i, e := range entries {
		ids[i] = e.StatusID
		scores[i] = e.Score
	}

	const q = `
		WITH new_rows AS (
			SELECT unnest($1::text[]) AS status_id,
			       unnest($2::float8[]) AS score
		)
		INSERT INTO trending_statuses (status_id, score, ranked_at)
		SELECT status_id, score, NOW() FROM new_rows
		ON CONFLICT (status_id) DO UPDATE
		    SET score = EXCLUDED.score, ranked_at = EXCLUDED.ranked_at`

	if _, err := s.pool.Exec(ctx, `DELETE FROM trending_statuses`); err != nil {
		return fmt.Errorf("ReplaceTrendingStatuses truncate: %w", mapErr(err))
	}
	if _, err := s.pool.Exec(ctx, q, ids, scores); err != nil {
		return fmt.Errorf("ReplaceTrendingStatuses upsert: %w", mapErr(err))
	}
	return nil
}

// GetTrendingStatusIDs returns up to limit entries from the trending_statuses index.
func (s *PostgresStore) GetTrendingStatusIDs(ctx context.Context, limit int) ([]domain.TrendingStatus, error) {
	const q = `
		SELECT status_id, score, ranked_at FROM trending_statuses
		ORDER BY score DESC LIMIT $1`

	rows, err := s.pool.Query(ctx, q, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("GetTrendingStatusIDs: %w", mapErr(err))
	}
	defer rows.Close()

	var out []domain.TrendingStatus
	for rows.Next() {
		var ts domain.TrendingStatus
		var rankedAt pgtype.Timestamptz
		if err := rows.Scan(&ts.StatusID, &ts.Score, &rankedAt); err != nil {
			return nil, fmt.Errorf("GetTrendingStatusIDs scan: %w", mapErr(err))
		}
		ts.RankedAt = pgTime(rankedAt)
		out = append(out, ts)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetTrendingStatusIDs rows: %w", mapErr(err))
	}
	return out, nil
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

	const q = `
		INSERT INTO trending_tag_history (hashtag_id, day, uses, accounts)
		SELECT unnest($1::text[]), unnest($2::date[]), unnest($3::bigint[]), unnest($4::bigint[])
		ON CONFLICT (hashtag_id, day) DO UPDATE
		    SET uses = EXCLUDED.uses, accounts = EXCLUDED.accounts`

	if _, err := s.pool.Exec(ctx, q, ids, days, uses, accounts); err != nil {
		return fmt.Errorf("UpsertTrendingTagHistory: %w", mapErr(err))
	}
	return nil
}

// GetLinkDailyStats returns per-URL per-day usage aggregates for the past `days` days.
func (s *PostgresStore) GetLinkDailyStats(ctx context.Context, days int) ([]domain.TrendingLinkStats, error) {
	const q = `
		SELECT url, day, uses, accounts
		FROM trending_link_history
		WHERE day >= CURRENT_DATE - ($1::int - 1)
		ORDER BY url, day DESC`

	rows, err := s.pool.Query(ctx, q, int64(days))
	if err != nil {
		return nil, fmt.Errorf("GetLinkDailyStats: %w", mapErr(err))
	}
	defer rows.Close()

	var out []domain.TrendingLinkStats
	for rows.Next() {
		var ls domain.TrendingLinkStats
		var day pgtype.Date
		if err := rows.Scan(&ls.URL, &day, &ls.Uses, &ls.Accounts); err != nil {
			return nil, fmt.Errorf("GetLinkDailyStats scan: %w", mapErr(err))
		}
		if day.Valid {
			ls.Day = day.Time
		}
		out = append(out, ls)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetLinkDailyStats rows: %w", mapErr(err))
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

	const q = `
		INSERT INTO trending_link_history (url, day, uses, accounts)
		SELECT unnest($1::text[]), unnest($2::date[]), unnest($3::bigint[]), unnest($4::bigint[])
		ON CONFLICT (url, day) DO UPDATE
		    SET uses = EXCLUDED.uses, accounts = EXCLUDED.accounts`

	if _, err := s.pool.Exec(ctx, q, urls, days, uses, accounts); err != nil {
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

	if _, err := tx.Exec(ctx, `DELETE FROM trending_links`); err != nil {
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

		const q = `
			INSERT INTO trending_links (url, score, ranked_at)
			SELECT unnest($1::text[]), unnest($2::float8[]), NOW()
			ON CONFLICT (url) DO UPDATE
			    SET score = EXCLUDED.score, ranked_at = EXCLUDED.ranked_at`

		if _, err := tx.Exec(ctx, q, urls, scores); err != nil {
			return fmt.Errorf("ReplaceTrendingLinks insert: %w", mapErr(err))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ReplaceTrendingLinks commit: %w", err)
	}
	return nil
}

// GetTrendingLinks returns trending links with up to `days` days of history,
// limited to `limit` links ordered by score descending.
func (s *PostgresStore) GetTrendingLinks(ctx context.Context, days int, limit int) ([]domain.TrendingLink, error) {
	const q = `
		SELECT tl.url, tlh.day, tlh.uses, tlh.accounts
		FROM trending_links tl
		LEFT JOIN trending_link_history tlh
		    ON tlh.url = tl.url AND tlh.day >= CURRENT_DATE - ($1::int - 1)
		ORDER BY tl.score DESC, tl.url, tlh.day DESC`

	rows, err := s.pool.Query(ctx, q, int64(days))
	if err != nil {
		return nil, fmt.Errorf("GetTrendingLinks: %w", mapErr(err))
	}
	defer rows.Close()

	type linkEntry struct {
		totalScore float64
		history    []domain.TrendingLinkHistoryDay
	}
	order := make([]string, 0)
	byURL := make(map[string]*linkEntry)

	for rows.Next() {
		var (
			url      string
			day      pgtype.Date
			uses     *int64
			accounts *int64
		)
		if err := rows.Scan(&url, &day, &uses, &accounts); err != nil {
			return nil, fmt.Errorf("GetTrendingLinks scan: %w", mapErr(err))
		}
		e, ok := byURL[url]
		if !ok {
			e = &linkEntry{}
			byURL[url] = e
			order = append(order, url)
		}
		if uses != nil && accounts != nil && day.Valid {
			e.history = append(e.history, domain.TrendingLinkHistoryDay{
				Day:      day.Time,
				Uses:     *uses,
				Accounts: *accounts,
			})
			e.totalScore += float64(*uses)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetTrendingLinks rows: %w", mapErr(err))
	}

	out := make([]domain.TrendingLink, 0, len(order))
	for _, url := range order {
		out = append(out, domain.TrendingLink{
			URL:     url,
			History: byURL[url].history,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// GetTrendingTags returns trending hashtags with up to `days` days of history,
// limited to `limit` hashtags ordered by total recent uses (descending).
func (s *PostgresStore) GetTrendingTags(ctx context.Context, days int, limit int) ([]domain.TrendingTag, error) {
	const q = `
		SELECT h.id, h.name, h.created_at, h.updated_at,
		       tth.day, tth.uses, tth.accounts
		FROM trending_tag_history tth
		JOIN hashtags h ON h.id = tth.hashtag_id
		WHERE tth.day >= CURRENT_DATE - ($1::int - 1)
		ORDER BY h.id, tth.day DESC`

	rows, err := s.pool.Query(ctx, q, int64(days))
	if err != nil {
		return nil, fmt.Errorf("GetTrendingTags: %w", mapErr(err))
	}
	defer rows.Close()

	type tagKey = string
	type tagEntry struct {
		hashtag   domain.Hashtag
		totalUses int64
		history   []domain.TagHistoryDay
	}
	order := make([]tagKey, 0)
	byID := make(map[tagKey]*tagEntry)

	for rows.Next() {
		var (
			hID, hName     string
			hCreatedAt     pgtype.Timestamptz
			hUpdatedAt     pgtype.Timestamptz
			day            pgtype.Date
			uses, accounts int64
		)
		if err := rows.Scan(&hID, &hName, &hCreatedAt, &hUpdatedAt, &day, &uses, &accounts); err != nil {
			return nil, fmt.Errorf("GetTrendingTags scan: %w", mapErr(err))
		}
		e, ok := byID[hID]
		if !ok {
			e = &tagEntry{
				hashtag: domain.Hashtag{
					ID:        hID,
					Name:      hName,
					CreatedAt: pgTime(hCreatedAt),
					UpdatedAt: pgTime(hUpdatedAt),
				},
			}
			byID[hID] = e
			order = append(order, hID)
		}
		var dayTime time.Time
		if day.Valid {
			dayTime = day.Time
		}
		e.totalUses += uses
		e.history = append(e.history, domain.TagHistoryDay{
			Day:      dayTime,
			Uses:     uses,
			Accounts: accounts,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetTrendingTags rows: %w", mapErr(err))
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
