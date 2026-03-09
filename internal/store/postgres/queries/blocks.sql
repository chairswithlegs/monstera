-- name: CreateBlock :one
INSERT INTO blocks (id, account_id, target_id) VALUES ($1, $2, $3) RETURNING *;

-- name: GetBlock :one
SELECT * FROM blocks WHERE account_id = $1 AND target_id = $2;

-- name: DeleteBlock :exec
DELETE FROM blocks WHERE account_id = $1 AND target_id = $2;

-- name: ListBlockedAccounts :many
SELECT a.* FROM accounts a
INNER JOIN blocks b ON b.target_id = a.id
WHERE b.account_id = $1
ORDER BY b.id DESC
LIMIT $2 OFFSET $3;

-- name: ListBlockedAccountsPaginated :many
SELECT b.id AS cursor, a.id, a.username, a.domain, a.display_name, a.note, a.public_key, a.private_key, a.inbox_url, a.outbox_url, a.followers_url, a.following_url, a.ap_id, a.ap_raw, a.bot, a.locked, a.suspended, a.silenced, a.created_at, a.updated_at, a.avatar_media_id, a.header_media_id, a.followers_count, a.following_count, a.statuses_count, a.fields
FROM accounts a
INNER JOIN blocks b ON b.target_id = a.id
WHERE b.account_id = $1
  AND ($2::text IS NULL OR b.id < $2)
ORDER BY b.id DESC
LIMIT $3;

-- name: IsBlocked :one
SELECT EXISTS(SELECT 1 FROM blocks WHERE account_id = $1 AND target_id = $2);

-- name: IsBlockedEitherDirection :one
SELECT EXISTS(
    SELECT 1 FROM blocks
    WHERE (account_id = $1 AND target_id = $2)
       OR (account_id = $2 AND target_id = $1)
);
