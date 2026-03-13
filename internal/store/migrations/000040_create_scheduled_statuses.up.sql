CREATE TABLE scheduled_statuses (
    id           TEXT PRIMARY KEY,
    account_id   TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    params       JSONB NOT NULL,
    scheduled_at TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scheduled_statuses_account ON scheduled_statuses (account_id);
CREATE INDEX idx_scheduled_statuses_scheduled_at ON scheduled_statuses (scheduled_at);
