DROP INDEX IF EXISTS idx_accounts_deletion_requested_at;

ALTER TABLE accounts DROP COLUMN deletion_requested_at;
