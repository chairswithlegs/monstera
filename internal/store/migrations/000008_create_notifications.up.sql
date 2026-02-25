CREATE TABLE notifications (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id),  -- recipient
    from_id    TEXT NOT NULL REFERENCES accounts(id),  -- actor
    type       TEXT NOT NULL,   -- 'follow'|'mention'|'reblog'|'favourite'|'follow_request'
    status_id  TEXT REFERENCES statuses(id) ON DELETE CASCADE,
    read       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary notification list query: recipient, newest first.
CREATE INDEX idx_notifications_account ON notifications (account_id, id DESC);

-- Dismiss and dedup checks: (account_id, from_id, type, status_id).
CREATE INDEX idx_notifications_dedup ON notifications (account_id, from_id, type, status_id);
