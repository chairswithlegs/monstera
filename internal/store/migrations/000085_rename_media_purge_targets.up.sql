-- Generalise the account_deletion_media_targets table so it can also hold
-- pending blob deletions from domain-block purges (issue #104). The schema
-- and lifecycle are already identical; the old name/column were tied to one
-- caller.
--
-- After this migration, purge_id is an opaque identifier owned by whichever
-- purge flow emitted the rows (account deletion or domain-block suspend).
-- The FK to account_deletion_snapshots is dropped because the new caller
-- does not have a snapshot row; rows are GC'd by the existing
-- purge-account-deletion-snapshots scheduler job which now also sweeps
-- rows whose delivered_at is older than 24h regardless of parent.

ALTER TABLE account_deletion_media_targets
    DROP CONSTRAINT account_deletion_media_targets_deletion_id_fkey;

ALTER TABLE account_deletion_media_targets
    RENAME COLUMN deletion_id TO purge_id;

ALTER TABLE account_deletion_media_targets
    RENAME CONSTRAINT account_deletion_media_targets_pkey TO media_purge_targets_pkey;

ALTER INDEX idx_account_deletion_media_targets_pending
    RENAME TO idx_media_purge_targets_pending;

ALTER TABLE account_deletion_media_targets
    RENAME TO media_purge_targets;
