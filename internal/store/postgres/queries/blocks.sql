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
SELECT b.id AS cursor, sqlc.embed(a)
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
