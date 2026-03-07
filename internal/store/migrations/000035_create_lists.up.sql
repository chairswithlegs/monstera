CREATE TABLE lists (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    replies_policy  TEXT NOT NULL DEFAULT 'list' CHECK (replies_policy IN ('followed', 'list', 'none')),
    exclusive       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lists_account ON lists (account_id);

CREATE TABLE list_accounts (
    list_id    TEXT NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    PRIMARY KEY (list_id, account_id)
);

CREATE INDEX idx_list_accounts_list ON list_accounts (list_id);
