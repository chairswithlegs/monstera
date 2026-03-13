ALTER TABLE users
    DROP COLUMN IF EXISTS default_quote_policy;

DROP TABLE IF EXISTS quote_approvals;

ALTER TABLE statuses
    DROP COLUMN IF EXISTS quoted_status_id,
    DROP COLUMN IF EXISTS quote_approval_policy,
    DROP COLUMN IF EXISTS quotes_count;
