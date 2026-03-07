-- name: CreateMute :one
INSERT INTO mutes (id, account_id, target_id, hide_notifications)
VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id, target_id) DO UPDATE SET hide_notifications = $4
RETURNING *;

-- name: GetMute :one
SELECT * FROM mutes WHERE account_id = $1 AND target_id = $2;

-- name: DeleteMute :exec
DELETE FROM mutes WHERE account_id = $1 AND target_id = $2;

-- name: ListMutes :many
SELECT * FROM mutes WHERE account_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3;

-- name: ListMutedAccountsPaginated :many
SELECT m.id AS cursor, a.id, a.username, a.domain, a.display_name, a.note, a.public_key, a.private_key, a.inbox_url, a.outbox_url, a.followers_url, a.following_url, a.ap_id, a.ap_raw, a.bot, a.locked, a.suspended, a.silenced, a.created_at, a.updated_at, a.avatar_media_id, a.header_media_id, a.followers_count, a.following_count, a.statuses_count, a.fields
FROM accounts a
INNER JOIN mutes m ON m.target_id = a.id
WHERE m.account_id = $1
  AND ($2::text IS NULL OR m.id < $2)
ORDER BY m.id DESC
LIMIT $3;

-- name: IsMuted :one
SELECT EXISTS(SELECT 1 FROM mutes WHERE account_id = $1 AND target_id = $2);
