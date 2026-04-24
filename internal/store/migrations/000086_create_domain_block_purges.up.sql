-- Tracks progress of the asynchronous account/status/media purge triggered
-- when an admin creates a domain block with severity=suspend (issue #104).
--
-- One row per in-flight or completed purge, keyed by the domain_blocks id.
-- The DomainBlockPurgeSubscriber processes one bounded batch of accounts per
-- NATS message and advances the cursor; when the batch is empty it marks the
-- row complete. CASCADE on block_id lets an admin undo the block by deleting
-- it; the subscriber sees the missing row on next delivery and stops.
--
-- cursor stores the last-processed account id for keyset pagination
-- (WHERE domain = $1 AND id > cursor).

CREATE TABLE domain_block_purges (
    block_id     TEXT PRIMARY KEY REFERENCES domain_blocks(id) ON DELETE CASCADE,
    domain       TEXT NOT NULL,
    cursor       TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_domain_block_purges_pending
    ON domain_block_purges (created_at)
    WHERE completed_at IS NULL;
