-- name: ListDomainBlocks :many
SELECT * FROM domain_blocks ORDER BY domain ASC;

-- name: GetDomainBlock :one
SELECT * FROM domain_blocks WHERE domain = $1;

-- name: CreateDomainBlock :one
INSERT INTO domain_blocks (id, domain, severity, reason)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateDomainBlock :one
UPDATE domain_blocks SET severity = $2, reason = $3 WHERE domain = $1 RETURNING *;

-- name: DeleteDomainBlock :exec
DELETE FROM domain_blocks WHERE domain = $1;
