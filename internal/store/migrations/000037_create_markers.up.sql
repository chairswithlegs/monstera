CREATE TABLE markers (
    account_id   TEXT NOT NULL REFERENCES accounts(id),
    timeline     TEXT NOT NULL,
    last_read_id TEXT NOT NULL DEFAULT '',
    version      INT NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, timeline)
);

CREATE INDEX idx_markers_account ON markers (account_id);
