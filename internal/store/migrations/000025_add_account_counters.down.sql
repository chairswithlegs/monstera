ALTER TABLE accounts
    DROP COLUMN IF EXISTS followers_count,
    DROP COLUMN IF EXISTS following_count,
    DROP COLUMN IF EXISTS statuses_count;
