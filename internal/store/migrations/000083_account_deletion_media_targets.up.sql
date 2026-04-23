-- Object-store blobs to purge after an account hard-delete.
--
-- media_attachments rows are CASCADE-deleted when the account row goes away
-- (migration 000080), but the underlying S3/local blobs are not touched by
-- Postgres. This table captures every storage_key owned by the account
-- BEFORE the DELETE fires, so a subscriber can iterate and call
-- mediaStore.Delete(key) per blob.
--
-- Mirrors account_deletion_targets (federation inbox delivery): same
-- deletion_id, same 24h lifecycle, same CASCADE-with-snapshot cleanup.

CREATE TABLE account_deletion_media_targets (
    deletion_id  TEXT NOT NULL REFERENCES account_deletion_snapshots(id) ON DELETE CASCADE,
    storage_key  TEXT NOT NULL,
    delivered_at TIMESTAMPTZ,
    PRIMARY KEY (deletion_id, storage_key)
);

CREATE INDEX idx_account_deletion_media_targets_pending
    ON account_deletion_media_targets (deletion_id, storage_key)
    WHERE delivered_at IS NULL;
