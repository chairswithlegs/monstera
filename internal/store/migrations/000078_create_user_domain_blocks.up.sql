CREATE TABLE user_domain_blocks (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    domain     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, domain)
);

CREATE INDEX idx_user_domain_blocks_account ON user_domain_blocks (account_id);
