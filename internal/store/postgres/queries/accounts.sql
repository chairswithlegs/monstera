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
    display_name    = $2,
    note            = $3,
    avatar_media_id = $4,
    header_media_id = $5,
    ap_raw          = $6,
    bot             = $7,
    locked          = $8,
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
