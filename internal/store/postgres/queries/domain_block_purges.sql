-- Queries against domain_block_purges — the tracker for the async purge
-- triggered by an admin creating a domain block with severity=suspend
-- (issue #104). One row per in-flight or completed purge, keyed by block_id.

-- name: CreateDomainBlockPurge :exec
INSERT INTO domain_block_purges (block_id, domain)
VALUES ($1, $2);

-- name: GetDomainBlockPurge :one
SELECT block_id, domain, cursor, created_at, completed_at
FROM domain_block_purges
WHERE block_id = $1;

-- name: UpdateDomainBlockPurgeCursor :exec
UPDATE domain_block_purges
SET cursor = $2
WHERE block_id = $1;

-- name: MarkDomainBlockPurgeComplete :exec
UPDATE domain_block_purges
SET completed_at = NOW()
WHERE block_id = $1;

-- name: ListDomainBlocksWithPurge :many
-- LEFT JOIN so silence-severity blocks (no purge row) still appear. The
-- optional accounts_remaining column is computed by the caller for
-- in-progress rows only; this query just surfaces the purge state.
SELECT
    b.id         AS block_id,
    b.domain     AS domain,
    b.severity   AS severity,
    b.reason     AS reason,
    b.created_at AS created_at,
    p.cursor     AS purge_cursor,
    p.created_at AS purge_created_at,
    p.completed_at AS purge_completed_at
FROM domain_blocks b
LEFT JOIN domain_block_purges p ON p.block_id = b.id
ORDER BY b.created_at DESC;
