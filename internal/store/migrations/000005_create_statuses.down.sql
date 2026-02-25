ALTER TABLE media_attachments DROP CONSTRAINT IF EXISTS fk_media_status;
DROP TABLE IF EXISTS statuses CASCADE;
