ALTER TABLE accounts
    ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN header_url TEXT NOT NULL DEFAULT '';

-- Backfill local accounts from media_attachments.
UPDATE accounts a
SET avatar_url = m.url
FROM media_attachments m
WHERE a.avatar_media_id = m.id;

UPDATE accounts a
SET header_url = m.url
FROM media_attachments m
WHERE a.header_media_id = m.id;
