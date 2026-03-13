package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
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
		return nil, fmt.Errorf("GetTopScoredPublicStatuses: %w", err)
	}
	defer rows.Close()

	var out []domain.TrendingStatus
	for rows.Next() {
		var ts domain.TrendingStatus
		if err := rows.Scan(&ts.StatusID, &ts.Score); err != nil {
			return nil, fmt.Errorf("GetTopScoredPublicStatuses scan: %w", err)
		}
		out = append(out, ts)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetTopScoredPublicStatuses rows: %w", err)
	}
	return out, nil
}

// GetHashtagDailyStats returns per-hashtag per-day usage aggregates since `since`.
func (s *PostgresStore) GetHashtagDailyStats(ctx context.Context, since time.Time) ([]store.HashtagDailyStats, error) {
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
		return nil, fmt.Errorf("GetHashtagDailyStats: %w", err)
	}
	defer rows.Close()

	var out []store.HashtagDailyStats
	for rows.Next() {
		var hs store.HashtagDailyStats
		var day pgtype.Date
		if err := rows.Scan(&hs.HashtagID, &hs.HashtagName, &day, &hs.Uses, &hs.Accounts); err != nil {
			return nil, fmt.Errorf("GetHashtagDailyStats scan: %w", err)
		}
		if day.Valid {
			hs.Day = day.Time
		}
		out = append(out, hs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetHashtagDailyStats rows: %w", err)
	}
	return out, nil
}

// ReplaceTrendingStatuses replaces the entire trending_statuses index atomically.
func (s *PostgresStore) ReplaceTrendingStatuses(ctx context.Context, entries []store.TrendingStatusEntry) error {
	if len(entries) == 0 {
		if _, err := s.pool.Exec(ctx, `DELETE FROM trending_statuses`); err != nil {
			return fmt.Errorf("ReplaceTrendingStatuses truncate: %w", err)
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
		return fmt.Errorf("ReplaceTrendingStatuses truncate: %w", err)
	}
	if _, err := s.pool.Exec(ctx, q, ids, scores); err != nil {
		return fmt.Errorf("ReplaceTrendingStatuses upsert: %w", err)
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
		return nil, fmt.Errorf("GetTrendingStatusIDs: %w", err)
	}
	defer rows.Close()

	var out []domain.TrendingStatus
	for rows.Next() {
		var ts domain.TrendingStatus
		var rankedAt pgtype.Timestamptz
		if err := rows.Scan(&ts.StatusID, &ts.Score, &rankedAt); err != nil {
			return nil, fmt.Errorf("GetTrendingStatusIDs scan: %w", err)
		}
		ts.RankedAt = pgTime(rankedAt)
		out = append(out, ts)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetTrendingStatusIDs rows: %w", err)
	}
	return out, nil
}

// UpsertTrendingTagHistory inserts or updates daily usage entries in trending_tag_history.
func (s *PostgresStore) UpsertTrendingTagHistory(ctx context.Context, entries []store.TrendingTagHistoryEntry) error {
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
		return fmt.Errorf("UpsertTrendingTagHistory: %w", err)
	}
	return nil
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
		return nil, fmt.Errorf("GetTrendingTags: %w", err)
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
			return nil, fmt.Errorf("GetTrendingTags scan: %w", err)
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
		return nil, fmt.Errorf("GetTrendingTags rows: %w", err)
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
