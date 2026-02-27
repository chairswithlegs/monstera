-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: GetLocalAccountByUsername :one
SELECT * FROM accounts WHERE username = $1 AND domain IS NULL;

-- name: GetRemoteAccountByUsername :one
SELECT * FROM accounts WHERE username = $1 AND domain = $2;

-- name: GetAccountByAPID :one
SELECT * FROM accounts WHERE ap_id = $1;

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
SELECT * FROM accounts WHERE domain IS NULL ORDER BY id DESC LIMIT $1 OFFSET $2;

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

-- name: DecrementStatusesCount :exec
UPDATE accounts SET statuses_count = GREATEST(0, statuses_count - 1) WHERE id = $1;

-- name: SearchAccounts :many
SELECT * FROM accounts
WHERE suspended = FALSE
  AND (
    LOWER(username) LIKE LOWER($1) || '%'
    OR (domain IS NOT NULL AND LOWER(username) || '@' || LOWER(COALESCE(domain, '')) LIKE LOWER($1) || '%')
  )
ORDER BY (domain IS NOT NULL), username
LIMIT $2;
