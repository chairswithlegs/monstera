CREATE TABLE blocks (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id),  -- blocker
    target_id  TEXT NOT NULL REFERENCES accounts(id),  -- blocked
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, target_id)
);

CREATE INDEX idx_blocks_account ON blocks (account_id);
-- Check "am I blocked by this person?" — needed before delivering content.
CREATE INDEX idx_blocks_target ON blocks (target_id);
