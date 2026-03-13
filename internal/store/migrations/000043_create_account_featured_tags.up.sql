CREATE TABLE account_featured_tags (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    tag_id     TEXT NOT NULL REFERENCES hashtags(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, tag_id)
);

CREATE INDEX idx_account_featured_tags_account ON account_featured_tags (account_id);
