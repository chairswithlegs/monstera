CREATE TABLE reports (
    id             TEXT PRIMARY KEY,
    account_id     TEXT NOT NULL REFERENCES accounts(id),  -- reporter
    target_id      TEXT NOT NULL REFERENCES accounts(id),  -- reported account
    status_ids     TEXT[],                                  -- optional reported post IDs
    comment        TEXT,
    category       TEXT NOT NULL DEFAULT 'other',           -- 'spam'|'illegal'|'violation'|'other'
    state          TEXT NOT NULL DEFAULT 'open',            -- 'open'|'resolved'
    assigned_to_id TEXT REFERENCES users(id),
    action_taken   TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at    TIMESTAMPTZ
);

-- Admin queue: open reports, newest first.
CREATE INDEX idx_reports_open ON reports (created_at DESC) WHERE state = 'open';
-- Reports assigned to a specific moderator.
CREATE INDEX idx_reports_assigned ON reports (assigned_to_id) WHERE assigned_to_id IS NOT NULL;
