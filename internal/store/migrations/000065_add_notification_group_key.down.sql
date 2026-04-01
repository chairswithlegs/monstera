DROP INDEX IF EXISTS idx_notifications_group_key;
ALTER TABLE notifications DROP COLUMN IF EXISTS group_key;
