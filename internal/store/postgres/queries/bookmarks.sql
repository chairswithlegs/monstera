-- name: CreateBookmark :exec
INSERT INTO bookmarks (id, account_id, status_id) VALUES ($1, $2, $3);

-- name: DeleteBookmark :exec
DELETE FROM bookmarks WHERE account_id = $1 AND status_id = $2;

-- name: IsBookmarked :one
SELECT EXISTS(SELECT 1 FROM bookmarks WHERE account_id = $1 AND status_id = $2);

-- name: GetBookmarksTimeline :many
SELECT b.id AS cursor, s.id, s.uri, s.account_id, s.text, s.content, s.content_warning, s.visibility, s.language, s.in_reply_to_id, s.reblog_of_id, s.ap_id, s.ap_raw, s.sensitive, s.local, s.edited_at, s.replies_count, s.reblogs_count, s.favourites_count, s.created_at, s.updated_at, s.deleted_at, s.in_reply_to_account_id
FROM bookmarks b
INNER JOIN statuses s ON s.id = b.status_id
WHERE b.account_id = $1 AND s.deleted_at IS NULL
  AND ($2::text IS NULL OR b.id < $2)
ORDER BY b.id DESC
LIMIT $3;
