CREATE TABLE bookmarks (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    status_id  TEXT NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, status_id)
);

CREATE INDEX idx_bookmarks_account ON bookmarks (account_id, id DESC);
