-- name: GetStatusByID :one
SELECT * FROM statuses WHERE id = $1 AND deleted_at IS NULL;

-- name: GetStatusByAPID :one
SELECT * FROM statuses WHERE ap_id = $1 AND deleted_at IS NULL;

-- name: CreateStatus :one
INSERT INTO statuses (
    id, uri, account_id, text, content, content_warning,
    visibility, language, in_reply_to_id, in_reply_to_account_id, reblog_of_id,
    ap_id, ap_raw, sensitive, local
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15
) RETURNING *;

-- name: UpdateStatus :one
UPDATE statuses SET
    text            = $2,
    content         = $3,
    content_warning = $4,
    sensitive       = $5,
    edited_at       = NOW(),
    updated_at      = NOW()
WHERE id = $1
RETURNING *;

-- name: SoftDeleteStatus :exec
UPDATE statuses SET deleted_at = NOW() WHERE id = $1;

-- name: GetHomeTimeline :many
SELECT s.* FROM statuses s
WHERE s.deleted_at IS NULL
  AND s.account_id IN (
      SELECT f.target_id FROM follows f
      WHERE f.account_id = $1 AND f.state = 'accepted'
      UNION ALL
      SELECT $1::text
  )
  AND ($2::text IS NULL OR s.id < $2)
ORDER BY s.id DESC
LIMIT $3;

-- name: GetPublicTimeline :many
SELECT * FROM statuses
WHERE deleted_at IS NULL
  AND visibility = 'public'
  AND ($1::boolean = FALSE OR local = TRUE)
  AND ($2::text IS NULL OR id < $2)
ORDER BY id DESC
LIMIT $3;

-- name: CountLocalStatuses :one
SELECT COUNT(*) FROM statuses WHERE local = TRUE AND deleted_at IS NULL;

-- name: CountAccountPublicStatuses :one
SELECT COUNT(*) FROM statuses
WHERE account_id = $1 AND deleted_at IS NULL AND visibility = 'public' AND reblog_of_id IS NULL;

-- name: GetAccountPublicStatuses :many
SELECT * FROM statuses
WHERE account_id = $1
  AND deleted_at IS NULL
  AND visibility = 'public'
  AND reblog_of_id IS NULL
  AND ($2::text IS NULL OR id < $2)
ORDER BY id DESC
LIMIT $3;

-- name: GetAccountStatuses :many
SELECT * FROM statuses
WHERE account_id = $1
  AND deleted_at IS NULL
  AND reblog_of_id IS NULL
  AND ($2::text IS NULL OR id < $2)
ORDER BY id DESC
LIMIT $3;

-- name: GetAccountStatusesWithBoosts :many
SELECT * FROM statuses
WHERE account_id = $1
  AND deleted_at IS NULL
  AND ($2::text IS NULL OR id < $2)
ORDER BY id DESC
LIMIT $3;

-- name: GetStatusAncestors :many
WITH RECURSIVE ancestors AS (
    SELECT st.* FROM statuses st WHERE st.id = (
        SELECT s2.in_reply_to_id FROM statuses s2 WHERE s2.id = $1 AND s2.deleted_at IS NULL
    )
    UNION ALL
    SELECT s.* FROM statuses s
    INNER JOIN ancestors a ON s.id = a.in_reply_to_id
    WHERE s.deleted_at IS NULL
)
SELECT * FROM ancestors ORDER BY id ASC;

-- name: GetStatusDescendants :many
WITH RECURSIVE descendants AS (
    SELECT st.* FROM statuses st WHERE st.in_reply_to_id = $1 AND st.deleted_at IS NULL
    UNION ALL
    SELECT s.* FROM statuses s
    INNER JOIN descendants d ON s.in_reply_to_id = d.id
    WHERE s.deleted_at IS NULL
)
SELECT * FROM descendants ORDER BY id ASC;

-- name: IncrementRepliesCount :exec
UPDATE statuses SET replies_count = replies_count + 1 WHERE id = $1;

-- name: DecrementRepliesCount :exec
UPDATE statuses SET replies_count = GREATEST(0, replies_count - 1) WHERE id = $1;

-- name: IncrementReblogsCount :exec
UPDATE statuses SET reblogs_count = reblogs_count + 1 WHERE id = $1;

-- name: DecrementReblogsCount :exec
UPDATE statuses SET reblogs_count = GREATEST(0, reblogs_count - 1) WHERE id = $1;

-- name: IncrementFavouritesCount :exec
UPDATE statuses SET favourites_count = favourites_count + 1 WHERE id = $1;

-- name: DecrementFavouritesCount :exec
UPDATE statuses SET favourites_count = GREATEST(0, favourites_count - 1) WHERE id = $1;

-- name: GetReblogByAccountAndTarget :one
SELECT * FROM statuses
WHERE account_id = $1 AND reblog_of_id = $2 AND deleted_at IS NULL;

-- name: GetRebloggedBy :many
SELECT a.id, a.username, a.domain, a.display_name, a.note, a.public_key, a.private_key, a.inbox_url, a.outbox_url, a.followers_url, a.following_url, a.ap_id, a.ap_raw, a.bot, a.locked, a.suspended, a.silenced, a.created_at, a.updated_at, a.avatar_media_id, a.header_media_id, a.followers_count, a.following_count, a.statuses_count, a.fields FROM accounts a
INNER JOIN statuses s ON s.account_id = a.id
WHERE s.reblog_of_id = $1 AND s.deleted_at IS NULL
  AND ($2::text IS NULL OR s.id < $2)
ORDER BY s.id DESC
LIMIT $3;

-- name: CreateStatusEdit :one
INSERT INTO status_edits (id, status_id, account_id, text, content, content_warning, sensitive)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListStatusEdits :many
SELECT * FROM status_edits WHERE status_id = $1 ORDER BY created_at ASC;
