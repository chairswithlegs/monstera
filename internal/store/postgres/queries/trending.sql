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
