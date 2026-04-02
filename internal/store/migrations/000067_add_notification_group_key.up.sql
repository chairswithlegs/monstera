ALTER TABLE notifications ADD COLUMN group_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_notifications_group_key ON notifications (account_id, group_key);
