ALTER TABLE accounts ADD COLUMN deletion_requested_at TIMESTAMPTZ;

-- Supports the scheduler purge job scan for accounts past their grace period.
CREATE INDEX idx_accounts_deletion_requested_at
    ON accounts (deletion_requested_at)
    WHERE deletion_requested_at IS NOT NULL;
