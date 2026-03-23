ALTER TABLE accounts
    DROP COLUMN IF EXISTS avatar_url,
    DROP COLUMN IF EXISTS header_url;
