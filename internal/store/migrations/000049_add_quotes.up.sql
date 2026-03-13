-- Quote posts: status can reference a quoted status; quote_approvals tracks revocations.
ALTER TABLE statuses
    ADD COLUMN quoted_status_id TEXT REFERENCES statuses(id),
    ADD COLUMN quote_approval_policy TEXT NOT NULL DEFAULT 'public',
    ADD COLUMN quotes_count INT NOT NULL DEFAULT 0;

CREATE INDEX idx_statuses_quoted ON statuses (quoted_status_id)
    WHERE quoted_status_id IS NOT NULL;

CREATE TABLE quote_approvals (
    quoting_status_id TEXT PRIMARY KEY REFERENCES statuses(id) ON DELETE CASCADE,
    quoted_status_id   TEXT NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
    revoked_at        TIMESTAMPTZ
);

CREATE INDEX idx_quote_approvals_quoted ON quote_approvals (quoted_status_id)
    WHERE revoked_at IS NULL;

ALTER TABLE users
    ADD COLUMN default_quote_policy TEXT NOT NULL DEFAULT 'public';
