-- name: CreateFavourite :one
INSERT INTO favourites (id, account_id, status_id, ap_id) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetFavouriteByAPID :one
SELECT * FROM favourites WHERE ap_id = $1;

-- name: GetFavouriteByAccountAndStatus :one
SELECT * FROM favourites WHERE account_id = $1 AND status_id = $2;

-- name: DeleteFavourite :exec
DELETE FROM favourites WHERE account_id = $1 AND status_id = $2;

-- name: IsFavourited :one
SELECT EXISTS(SELECT 1 FROM favourites WHERE account_id = $1 AND status_id = $2);

-- name: GetStatusFavouritedBy :many
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
INNER JOIN favourites f ON f.account_id = a.id
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE f.status_id = $1
  AND ($2::text IS NULL OR f.id < $2)
ORDER BY f.id DESC
LIMIT $3;

-- name: GetFavouritesTimeline :many
SELECT f.id AS cursor, s.id, s.uri, s.account_id, s.text, s.content, s.content_warning, s.visibility, s.language, s.in_reply_to_id, s.reblog_of_id, s.ap_id, s.ap_raw, s.sensitive, s.local, s.edited_at, s.replies_count, s.reblogs_count, s.favourites_count, s.created_at, s.updated_at, s.deleted_at, s.in_reply_to_account_id
FROM favourites f
INNER JOIN statuses s ON s.id = f.status_id
WHERE f.account_id = $1 AND s.deleted_at IS NULL
  AND ($2::text IS NULL OR f.id < $2)
ORDER BY f.id DESC
LIMIT $3;
