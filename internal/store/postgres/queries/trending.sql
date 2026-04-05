-- name: TruncateTrendingStatuses :exec
DELETE FROM trending_statuses;

-- name: BulkUpsertTrendingStatuses :exec
INSERT INTO trending_statuses (status_id, score, ranked_at)
SELECT unnest($1::text[]), unnest($2::float8[]), NOW()
ON CONFLICT (status_id) DO UPDATE
    SET score = EXCLUDED.score, ranked_at = EXCLUDED.ranked_at;

-- name: GetTrendingStatuses :many
SELECT status_id, score, ranked_at FROM trending_statuses
ORDER BY score DESC LIMIT $1;

-- name: UpsertTrendingTagHistory :exec
INSERT INTO trending_tag_history (hashtag_id, day, uses, accounts)
SELECT unnest($1::text[]), unnest($2::date[]), unnest($3::bigint[]), unnest($4::bigint[])
ON CONFLICT (hashtag_id, day) DO UPDATE
    SET uses = EXCLUDED.uses, accounts = EXCLUDED.accounts;

-- name: GetTrendingTagHistory :many
SELECT h.id, h.name, h.created_at, h.updated_at,
       tth.day, tth.uses, tth.accounts
FROM trending_tag_history tth
JOIN hashtags h ON h.id = tth.hashtag_id
WHERE tth.day >= CURRENT_DATE - ($1::int - 1)
ORDER BY h.id, tth.day DESC;

-- name: GetLinkDailyStats :many
SELECT sc.url,
       date_trunc('day', s.created_at AT TIME ZONE 'UTC')::date AS day,
       COUNT(*)                     AS uses,
       COUNT(DISTINCT s.account_id) AS accounts
FROM status_cards sc
JOIN statuses s ON s.id = sc.status_id
JOIN accounts a ON a.id = s.account_id
WHERE s.deleted_at IS NULL
  AND s.visibility IN ('public', 'unlisted')
  AND s.created_at >= CURRENT_DATE - ($1::int - 1)
  AND ($2::boolean = FALSE OR s.local = TRUE)
  AND NOT EXISTS (
      SELECT 1 FROM domain_blocks db
      WHERE db.domain = a.domain AND db.severity = 'silence'
  )
GROUP BY sc.url, day
ORDER BY sc.url, day DESC;

-- name: UpsertTrendingLinkHistory :exec
INSERT INTO trending_link_history (url, day, uses, accounts)
SELECT unnest($1::text[]), unnest($2::date[]), unnest($3::bigint[]), unnest($4::bigint[])
ON CONFLICT (url, day) DO UPDATE
    SET uses = EXCLUDED.uses, accounts = EXCLUDED.accounts;

-- name: ReplaceTrendingLinks :exec
DELETE FROM trending_links;

-- name: BulkInsertTrendingLinks :exec
INSERT INTO trending_links (url, score, ranked_at)
SELECT unnest($1::text[]), unnest($2::float8[]), NOW()
ON CONFLICT (url) DO UPDATE
    SET score = EXCLUDED.score, ranked_at = EXCLUDED.ranked_at;

-- name: GetTrendingLinks :many
WITH ranked_links AS (
    SELECT tl.url, tl.score,
           sc.title, sc.description, sc.card_type, sc.provider_name,
           sc.provider_url, sc.image_url, sc.width, sc.height
    FROM trending_links tl
    LEFT JOIN LATERAL (
        SELECT sc.title, sc.description, sc.card_type, sc.provider_name,
               sc.provider_url, sc.image_url, sc.width, sc.height
        FROM status_cards sc
        WHERE sc.url = tl.url
          AND sc.processing_state = 'fetched'
        ORDER BY sc.fetched_at DESC
        LIMIT 1
    ) sc ON true
    ORDER BY tl.score DESC
    LIMIT $2
)
SELECT rl.url, rl.title, rl.description, rl.card_type,
       rl.provider_name, rl.provider_url, rl.image_url,
       rl.width, rl.height,
       tlh.day, tlh.uses, tlh.accounts
FROM ranked_links rl
LEFT JOIN trending_link_history tlh
    ON tlh.url = rl.url AND tlh.day >= CURRENT_DATE - ($1::int - 1)
ORDER BY rl.score DESC, rl.url, tlh.day DESC;

-- name: GetTopScoredPublicStatuses :many
SELECT s.id AS status_id,
       (s.reblogs_count + s.favourites_count + s.replies_count * 0.5)::float8 AS score
FROM statuses s
JOIN accounts a ON a.id = s.account_id
WHERE s.deleted_at IS NULL
  AND s.visibility = 'public'
  AND s.reblog_of_id IS NULL
  AND s.created_at >= @since
  AND (@local_only::boolean = FALSE OR s.local = TRUE)
  AND NOT EXISTS (
      SELECT 1 FROM domain_blocks db
      WHERE db.domain = a.domain AND db.severity = 'silence'
  )
ORDER BY score DESC
LIMIT @max_results;

-- name: GetHashtagDailyStats :many
SELECT h.id AS hashtag_id, h.name AS hashtag_name,
       date_trunc('day', s.created_at AT TIME ZONE 'UTC')::date AS day,
       COUNT(*) AS uses,
       COUNT(DISTINCT s.account_id) AS accounts
FROM status_hashtags sh
JOIN statuses  s ON s.id  = sh.status_id
JOIN accounts  a ON a.id  = s.account_id
JOIN hashtags  h ON h.id  = sh.hashtag_id
WHERE s.deleted_at IS NULL
  AND s.visibility IN ('public', 'unlisted')
  AND s.created_at >= @since
  AND (@local_only::boolean = FALSE OR s.local = TRUE)
  AND NOT EXISTS (
      SELECT 1 FROM domain_blocks db
      WHERE db.domain = a.domain AND db.severity = 'silence'
  )
GROUP BY h.id, h.name, day
ORDER BY day DESC, uses DESC;

-- name: TruncateTrendingTagHistory :exec
DELETE FROM trending_tag_history;

-- name: AddTrendingLinkFilter :exec
INSERT INTO trending_link_filters (url) VALUES ($1)
ON CONFLICT (url) DO NOTHING;

-- name: RemoveTrendingLinkFilter :exec
DELETE FROM trending_link_filters WHERE url = $1;

-- name: ListTrendingLinkFilters :many
SELECT url, created_at FROM trending_link_filters ORDER BY created_at DESC;

-- name: IsTrendingLinkFiltered :one
SELECT EXISTS(SELECT 1 FROM trending_link_filters WHERE url = $1)::boolean;
