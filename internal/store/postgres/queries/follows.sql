-- name: GetFollow :one
SELECT * FROM follows WHERE account_id = $1 AND target_id = $2;

-- name: GetFollowByID :one
SELECT * FROM follows WHERE id = $1;

-- name: GetFollowByAPID :one
SELECT * FROM follows WHERE ap_id = $1;

-- name: CreateFollow :one
INSERT INTO follows (id, account_id, target_id, state, ap_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: AcceptFollow :exec
UPDATE follows SET state = 'accepted' WHERE id = $1;

-- name: DeleteFollow :exec
DELETE FROM follows WHERE account_id = $1 AND target_id = $2;

-- name: GetFollowers :many
SELECT sqlc.embed(a)
FROM accounts a
INNER JOIN follows f ON f.account_id = a.id
WHERE f.target_id = $1
  AND f.state = 'accepted'
  AND ($2::text IS NULL OR f.id < $2)
ORDER BY f.id DESC
LIMIT $3;

-- name: GetFollowing :many
SELECT sqlc.embed(a)
FROM accounts a
INNER JOIN follows f ON f.target_id = a.id
WHERE f.account_id = $1
  AND f.state = 'accepted'
  AND ($2::text IS NULL OR f.id < $2)
ORDER BY f.id DESC
LIMIT $3;

-- name: GetFollowerInboxURLs :many
SELECT a.inbox_url FROM accounts a
INNER JOIN follows f ON f.account_id = a.id
WHERE f.target_id = $1
  AND f.state = 'accepted'
  AND a.domain IS NOT NULL
  AND a.suspended = FALSE;

-- name: GetDistinctFollowerInboxURLsPaginated :many
SELECT DISTINCT a.inbox_url FROM accounts a
INNER JOIN follows f ON f.account_id = a.id
WHERE f.target_id = $1
  AND f.state = 'accepted'
  AND a.domain IS NOT NULL
  AND a.suspended = FALSE
  AND ($2::text IS NULL OR a.inbox_url > $2)
ORDER BY a.inbox_url
LIMIT $3;

-- name: GetPendingFollowRequests :many
SELECT f.*, a.username, a.display_name, a.ap_id FROM follows f
INNER JOIN accounts a ON a.id = f.account_id
WHERE f.target_id = $1 AND f.state = 'pending'
ORDER BY f.created_at ASC;

-- name: GetPendingFollowRequestsPaginated :many
SELECT f.id AS cursor, sqlc.embed(a)
FROM follows f
INNER JOIN accounts a ON a.id = f.account_id
WHERE f.target_id = $1 AND f.state = 'pending'
  AND ($2::text IS NULL OR f.id < $2)
ORDER BY f.id DESC
LIMIT $3;

-- name: CountFollowers :one
SELECT COUNT(*) FROM follows WHERE target_id = $1 AND state = 'accepted';

-- name: CountFollowing :one
SELECT COUNT(*) FROM follows WHERE account_id = $1 AND state = 'accepted';

-- name: GetLocalFollowerAccountIDs :many
SELECT f.account_id FROM follows f
INNER JOIN accounts a ON a.id = f.account_id
WHERE f.target_id = $1
  AND f.state = 'accepted'
  AND a.domain IS NULL
  AND a.suspended = FALSE;

-- name: DeleteFollowsByDomain :exec
DELETE FROM follows
WHERE account_id IN (SELECT a.id FROM accounts a WHERE a.domain = $1)
   OR target_id IN (SELECT a.id FROM accounts a WHERE a.domain = $1);
