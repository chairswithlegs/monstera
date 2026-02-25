ALTER TABLE accounts
    ADD COLUMN avatar_media_id TEXT REFERENCES media_attachments(id) ON DELETE SET NULL,
    ADD COLUMN header_media_id TEXT REFERENCES media_attachments(id) ON DELETE SET NULL;
