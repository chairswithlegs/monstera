CREATE TABLE announcement_reads (
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    announcement_id TEXT NOT NULL REFERENCES announcements(id),
    read_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, announcement_id)
);

CREATE INDEX idx_announcement_reads_account ON announcement_reads (account_id);
