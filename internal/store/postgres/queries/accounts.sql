-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: GetAccountByIDForUpdate :one
SELECT * FROM accounts WHERE id = $1 FOR UPDATE;

-- name: GetAccountsByIDs :many
SELECT * FROM accounts WHERE id = ANY($1::text[]);

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
    ap_id, bot, locked, url,
    avatar_url, header_url,
    followers_count, following_count, statuses_count,
    featured_url
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7,
    $8, $9, $10, $11,
    $12, $13, $14, $15,
    $16, $17,
    $18, $19, $20,
    $21
) RETURNING *;

-- name: UpdateAccount :one
UPDATE accounts SET
    display_name    = COALESCE($2, display_name),
    note            = COALESCE($3, note),
    avatar_media_id = COALESCE($4, avatar_media_id),
    header_media_id = COALESCE($5, header_media_id),
    bot             = COALESCE($6, bot),
    locked          = COALESCE($7, locked),
    fields          = COALESCE($8, fields),
    url             = COALESCE($9, url),
    avatar_url      = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    header_url      = COALESCE(sqlc.narg('header_url'), header_url),
    updated_at      = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateAccountKeys :exec
UPDATE accounts SET public_key = $2, updated_at = NOW() WHERE id = $1;

-- name: UpdateAccountURLs :exec
UPDATE accounts SET inbox_url = $2, outbox_url = $3, followers_url = $4, following_url = $5, updated_at = NOW() WHERE id = $1;

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

-- name: UpdateAccountLastStatusAt :exec
UPDATE accounts SET last_status_at = NOW() WHERE id = $1;

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
LIMIT $2
OFFSET $3;

-- name: SearchAccountsFollowing :many
SELECT a.* FROM accounts a
INNER JOIN follows f ON f.target_id = a.id
WHERE f.account_id = $1
  AND a.suspended = FALSE
  AND f.state = 'accepted'
  AND (
    LOWER(a.username) LIKE LOWER($2) || '%'
    OR (a.domain IS NOT NULL AND LOWER(a.username) || '@' || LOWER(COALESCE(a.domain, '')) LIKE LOWER($2) || '%')
  )
ORDER BY (a.domain IS NOT NULL), a.username
LIMIT $3
OFFSET $4;

-- name: ListDirectoryAccounts :many
SELECT * FROM accounts
WHERE (NOT $1 OR domain IS NULL)
  AND suspended = FALSE
ORDER BY
  CASE WHEN $2::text = 'active' THEN last_status_at END DESC NULLS LAST,
  created_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdateRemoteAccountMeta :exec
UPDATE accounts SET
    avatar_url      = $2,
    header_url      = $3,
    followers_count = $4,
    following_count = $5,
    statuses_count  = $6,
    featured_url    = $7,
    updated_at      = NOW()
WHERE id = $1;

-- name: UpdateAccountLastBackfilledAt :exec
UPDATE accounts SET last_backfilled_at = @last_backfilled_at WHERE id = @id;


-- name: DeleteAccount :one
DELETE FROM accounts WHERE id = $1 RETURNING *;

-- name: ListRemoteAccountsByDomainPaginated :many
-- Keyset pagination over remote accounts on a given domain. Used by the
-- domain-block purge subscriber; $2 is the exclusive cursor (last processed
-- id). Pass '' to start at the beginning.
SELECT id
FROM accounts
WHERE domain = $1
  AND ($2::text IS NULL OR $2::text = '' OR id > $2)
ORDER BY id
LIMIT $3;

-- name: CountRemoteAccountsByDomainAfterCursor :one
-- Used by the admin API to compute "accounts remaining" for an in-progress
-- domain purge. Pass '' to count all accounts on the domain.
SELECT COUNT(*)
FROM accounts
WHERE domain = $1
  AND ($2::text IS NULL OR $2::text = '' OR id > $2);

-- name: SetAccountsDomainSuspendedByDomain :execrows
-- Bulk flip accounts.domain_suspended for every account on a domain. Called
-- in lockstep with CreateDomainBlock (on=true) and DeleteDomainBlock
-- (on=false) so visibility flips atomically with the block row. Returns the
-- count of rows updated for logging/audit.
UPDATE accounts SET domain_suspended = $2 WHERE domain = $1;
