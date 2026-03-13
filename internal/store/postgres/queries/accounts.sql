-- name: GetAccountByID :one
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE a.id = $1;

-- name: GetAccountsByIDs :many
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE a.id = ANY($1::text[]);

-- name: GetLocalAccountByUsername :one
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE a.username = $1 AND a.domain IS NULL;

-- name: GetRemoteAccountByUsername :one
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE a.username = $1 AND a.domain = $2;

-- name: GetAccountByAPID :one
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE a.ap_id = $1;

-- name: CreateAccount :one
INSERT INTO accounts (
    id, username, domain, display_name, note,
    public_key, private_key,
    inbox_url, outbox_url, followers_url, following_url,
    ap_id, ap_raw, bot, locked
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7,
    $8, $9, $10, $11,
    $12, $13, $14, $15
) RETURNING *;

-- name: UpdateAccount :one
UPDATE accounts SET
    display_name    = COALESCE($2, display_name),
    note            = COALESCE($3, note),
    avatar_media_id = COALESCE($4, avatar_media_id),
    header_media_id = COALESCE($5, header_media_id),
    ap_raw          = COALESCE($6, ap_raw),
    bot             = COALESCE($7, bot),
    locked          = COALESCE($8, locked),
    fields          = COALESCE($9, fields),
    updated_at      = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateAccountKeys :exec
UPDATE accounts SET public_key = $2, ap_raw = $3, updated_at = NOW() WHERE id = $1;

-- name: SuspendAccount :exec
UPDATE accounts SET suspended = TRUE, updated_at = NOW() WHERE id = $1;

-- name: UnsuspendAccount :exec
UPDATE accounts SET suspended = FALSE, updated_at = NOW() WHERE id = $1;

-- name: SilenceAccount :exec
UPDATE accounts SET silenced = TRUE, updated_at = NOW() WHERE id = $1;

-- name: UnsilenceAccount :exec
UPDATE accounts SET silenced = FALSE, updated_at = NOW() WHERE id = $1;

-- name: ListLocalAccounts :many
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE a.domain IS NULL ORDER BY a.id DESC LIMIT $1 OFFSET $2;

-- name: CountLocalAccounts :one
SELECT COUNT(*) FROM accounts WHERE domain IS NULL;

-- name: CountRemoteAccounts :one
SELECT COUNT(*) FROM accounts WHERE domain IS NOT NULL;

-- name: IncrementFollowersCount :exec
UPDATE accounts SET followers_count = followers_count + 1 WHERE id = $1;

-- name: DecrementFollowersCount :exec
UPDATE accounts SET followers_count = GREATEST(0, followers_count - 1) WHERE id = $1;

-- name: IncrementFollowingCount :exec
UPDATE accounts SET following_count = following_count + 1 WHERE id = $1;

-- name: DecrementFollowingCount :exec
UPDATE accounts SET following_count = GREATEST(0, following_count - 1) WHERE id = $1;

-- name: IncrementStatusesCount :exec
UPDATE accounts SET statuses_count = statuses_count + 1 WHERE id = $1;

-- name: UpdateAccountLastStatusAt :exec
UPDATE accounts SET last_status_at = NOW() WHERE id = $1;

-- name: DecrementStatusesCount :exec
UPDATE accounts SET statuses_count = GREATEST(0, statuses_count - 1) WHERE id = $1;

-- name: SearchAccounts :many
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE a.suspended = FALSE
  AND (
    LOWER(a.username) LIKE LOWER($1) || '%'
    OR (a.domain IS NOT NULL AND LOWER(a.username) || '@' || LOWER(COALESCE(a.domain, '')) LIKE LOWER($1) || '%')
  )
ORDER BY (a.domain IS NOT NULL), a.username
LIMIT $2;

-- name: ListDirectoryAccounts :many
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE (NOT $1 OR a.domain IS NULL)
  AND a.suspended = FALSE
ORDER BY
  CASE WHEN $2::text = 'active' THEN a.last_status_at END DESC NULLS LAST,
  a.created_at DESC
LIMIT $3 OFFSET $4;

-- name: DeleteAccount :exec
DELETE FROM accounts WHERE id = $1;
