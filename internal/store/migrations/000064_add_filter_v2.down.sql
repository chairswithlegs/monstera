DROP TABLE IF EXISTS user_filter_statuses;
DROP TABLE IF EXISTS user_filter_keywords;

ALTER TABLE user_filters
    DROP COLUMN IF EXISTS filter_action,
    DROP COLUMN IF EXISTS title;
