CREATE TABLE account_pins (
    account_id TEXT NOT NULL REFERENCES accounts(id),
    status_id   TEXT NOT NULL REFERENCES statuses(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, status_id)
);

CREATE INDEX idx_account_pins_account ON account_pins (account_id);
