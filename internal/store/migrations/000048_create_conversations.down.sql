DROP INDEX IF EXISTS idx_account_conversations_account;
DROP TABLE IF EXISTS account_conversations CASCADE;

DROP INDEX IF EXISTS idx_statuses_conversation_id;
ALTER TABLE statuses DROP COLUMN IF EXISTS conversation_id;

DROP TABLE IF EXISTS conversations CASCADE;
