DROP INDEX IF EXISTS idx_accounts_last_status_at;
ALTER TABLE accounts DROP COLUMN IF EXISTS last_status_at;
