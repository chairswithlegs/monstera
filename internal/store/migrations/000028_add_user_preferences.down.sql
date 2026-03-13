ALTER TABLE users
    DROP COLUMN IF EXISTS default_privacy,
    DROP COLUMN IF EXISTS default_sensitive,
    DROP COLUMN IF EXISTS default_language;
