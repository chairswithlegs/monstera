CREATE TABLE mutes (
    id                 TEXT PRIMARY KEY,
    account_id         TEXT NOT NULL REFERENCES accounts(id),
    target_id          TEXT NOT NULL REFERENCES accounts(id),
    hide_notifications BOOLEAN NOT NULL DEFAULT TRUE,  -- also suppress notifications from target
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, target_id)
);

CREATE INDEX idx_mutes_account ON mutes (account_id);
