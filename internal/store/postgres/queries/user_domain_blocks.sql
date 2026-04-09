-- name: CreateUserDomainBlock :exec
INSERT INTO user_domain_blocks (id, account_id, domain)
VALUES ($1, $2, $3)
ON CONFLICT (account_id, domain) DO NOTHING;

-- name: DeleteUserDomainBlock :exec
DELETE FROM user_domain_blocks WHERE account_id = $1 AND domain = $2;

-- name: ListUserDomainBlocksPaginated :many
SELECT id AS cursor, domain
FROM user_domain_blocks
WHERE account_id = $1
  AND ($2::text IS NULL OR id < $2)
ORDER BY id DESC
LIMIT $3;

-- name: IsUserDomainBlocked :one
SELECT EXISTS(SELECT 1 FROM user_domain_blocks WHERE account_id = $1 AND domain = $2);

-- name: DeleteFollowersByDomain :exec
DELETE FROM follows
WHERE target_id = $1
  AND account_id IN (SELECT id FROM accounts WHERE domain = $2);
