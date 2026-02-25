-- name: UpsertKnownInstance :exec
INSERT INTO known_instances (id, domain)
VALUES ($1, $2)
ON CONFLICT (domain) DO UPDATE SET last_seen_at = NOW();

-- name: UpdateKnownInstanceSoftware :exec
UPDATE known_instances SET software = $2, software_version = $3
WHERE domain = $1;

-- name: ListKnownInstances :many
SELECT ki.*,
    (SELECT COUNT(*) FROM accounts a WHERE a.domain = ki.domain) AS accounts_count
FROM known_instances ki
ORDER BY ki.last_seen_at DESC
LIMIT $1 OFFSET $2;

-- name: CountKnownInstances :one
SELECT COUNT(*) FROM known_instances;
