-- name: GetOrCreateHashtag :one
INSERT INTO hashtags (id, name)
VALUES ($1, lower($2))
ON CONFLICT (name) DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: GetHashtagByName :one
SELECT * FROM hashtags WHERE name = lower($1);

-- name: AttachHashtagsToStatus :exec
INSERT INTO status_hashtags (status_id, hashtag_id)
SELECT $1, unnest($2::text[])
ON CONFLICT DO NOTHING;

-- name: GetStatusHashtags :many
SELECT h.* FROM hashtags h
INNER JOIN status_hashtags sh ON sh.hashtag_id = h.id
WHERE sh.status_id = $1;

-- name: GetHashtagTimeline :many
SELECT s.* FROM statuses s
INNER JOIN status_hashtags sh ON sh.status_id = s.id
INNER JOIN hashtags h ON h.id = sh.hashtag_id
WHERE h.name = lower($1)
  AND s.deleted_at IS NULL
  AND s.visibility IN ('public', 'unlisted')
  AND ($2::text IS NULL OR s.id < $2)
ORDER BY s.id DESC
LIMIT $3;

-- name: SearchHashtagsByPrefix :many
SELECT * FROM hashtags
WHERE LOWER(name) LIKE LOWER($1) || '%'
ORDER BY name
LIMIT $2;
