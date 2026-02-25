CREATE TABLE status_mentions (
    status_id  TEXT NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    PRIMARY KEY (status_id, account_id)
);

CREATE INDEX idx_status_mentions_account ON status_mentions (account_id);
