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
SELECT url, day, uses, accounts
FROM trending_link_history
WHERE day >= CURRENT_DATE - ($1::int - 1)
ORDER BY url, day DESC;

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
SELECT url, score, ranked_at FROM trending_links
ORDER BY score DESC LIMIT $1;

-- name: GetTrendingLinkHistory :many
SELECT url, day, uses, accounts
FROM trending_link_history
WHERE url = $1 AND day >= CURRENT_DATE - ($2::int - 1)
ORDER BY day DESC;

-- name: AddTrendingLinkDenylist :exec
INSERT INTO trending_link_denylist (url) VALUES ($1)
ON CONFLICT (url) DO NOTHING;

-- name: RemoveTrendingLinkDenylist :exec
DELETE FROM trending_link_denylist WHERE url = $1;

-- name: ListTrendingLinkDenylist :many
SELECT url, created_at FROM trending_link_denylist ORDER BY created_at DESC;

-- name: IsTrendingLinkDenylisted :one
SELECT EXISTS(SELECT 1 FROM trending_link_denylist WHERE url = $1)::boolean;
