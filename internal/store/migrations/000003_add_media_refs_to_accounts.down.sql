ALTER TABLE accounts
    DROP COLUMN IF EXISTS avatar_media_id,
    DROP COLUMN IF EXISTS header_media_id;
