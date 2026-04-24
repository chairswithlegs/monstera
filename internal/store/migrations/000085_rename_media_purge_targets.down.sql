-- Reverse the rename in 000085. Restores the FK to account_deletion_snapshots.
-- Any rows whose purge_id does not match an existing snapshot must be cleaned
-- up before rolling back; otherwise the FK re-add fails.

ALTER TABLE media_purge_targets
    RENAME TO account_deletion_media_targets;

ALTER INDEX idx_media_purge_targets_pending
    RENAME TO idx_account_deletion_media_targets_pending;

ALTER TABLE account_deletion_media_targets
    RENAME CONSTRAINT media_purge_targets_pkey TO account_deletion_media_targets_pkey;

ALTER TABLE account_deletion_media_targets
    RENAME COLUMN purge_id TO deletion_id;

ALTER TABLE account_deletion_media_targets
    ADD CONSTRAINT account_deletion_media_targets_deletion_id_fkey
        FOREIGN KEY (deletion_id)
        REFERENCES account_deletion_snapshots(id) ON DELETE CASCADE;
