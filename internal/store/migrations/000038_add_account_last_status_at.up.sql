ALTER TABLE accounts ADD COLUMN last_status_at TIMESTAMPTZ;

CREATE INDEX idx_accounts_last_status_at ON accounts (last_status_at DESC NULLS LAST) WHERE domain IS NULL;
