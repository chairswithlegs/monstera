-- Add v2 filter fields to user_filters and create keyword/status sub-tables.

ALTER TABLE user_filters
    ADD COLUMN title        TEXT NOT NULL DEFAULT '',
    ADD COLUMN filter_action TEXT NOT NULL DEFAULT 'warn';

CREATE TABLE user_filter_keywords (
    id          TEXT PRIMARY KEY,
    filter_id   TEXT NOT NULL REFERENCES user_filters(id) ON DELETE CASCADE,
    keyword     TEXT NOT NULL,
    whole_word  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_filter_keywords_filter ON user_filter_keywords (filter_id);

CREATE TABLE user_filter_statuses (
    id          TEXT PRIMARY KEY,
    filter_id   TEXT NOT NULL REFERENCES user_filters(id) ON DELETE CASCADE,
    status_id   TEXT NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (filter_id, status_id)
);

CREATE INDEX idx_user_filter_statuses_filter ON user_filter_statuses (filter_id);

-- Migrate existing v1 filter phrases into the keywords table.
INSERT INTO user_filter_keywords (id, filter_id, keyword, whole_word, created_at)
SELECT gen_random_uuid()::text, id, phrase, whole_word, created_at
FROM user_filters
WHERE phrase != '';
