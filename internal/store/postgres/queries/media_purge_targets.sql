-- Queries against media_purge_targets — the shared pending-blob-delete queue
-- used by both account deletion and domain-block suspend. The purge_id column
-- is an opaque identifier owned by the purge flow that emitted the row; there
-- is no FK, so callers are responsible for cleanup via delivered_at sweep.

-- name: InsertMediaPurgeTargetsForAccount :exec
INSERT INTO media_purge_targets (purge_id, storage_key)
SELECT $1, storage_key
FROM media_attachments
WHERE account_id = $2
ON CONFLICT (purge_id, storage_key) DO NOTHING;

-- name: ListPendingMediaPurgeTargets :many
SELECT storage_key
FROM media_purge_targets
WHERE purge_id = $1
  AND delivered_at IS NULL
  AND ($2::text IS NULL OR $2::text = '' OR storage_key > $2)
ORDER BY storage_key
LIMIT $3;

-- name: MarkMediaPurgeTargetDelivered :exec
UPDATE media_purge_targets
SET delivered_at = NOW()
WHERE purge_id = $1 AND storage_key = $2;

-- name: DeleteDeliveredMediaPurgeTargets :execrows
-- Sweeps rows whose blobs have been deleted, past the given cutoff. Runs in
-- the same scheduler job that expires account_deletion_snapshots; compensates
-- for the FK removal between account_deletion_snapshots and the (now generic)
-- media_purge_targets table.
DELETE FROM media_purge_targets
WHERE delivered_at IS NOT NULL AND delivered_at < $1;
