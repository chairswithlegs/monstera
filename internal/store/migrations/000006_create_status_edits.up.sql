-- Stores a snapshot of each status revision so clients can show edit history.
-- A row is inserted before applying any Update{Note} or local edit.
CREATE TABLE status_edits (
    id              TEXT PRIMARY KEY,
    status_id       TEXT NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    text            TEXT,
    content         TEXT,                       -- rendered HTML at time of edit
    content_warning TEXT,
    sensitive       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Edit history is always fetched ordered oldest-first for a given status.
CREATE INDEX idx_status_edits_status ON status_edits (status_id, created_at ASC);
