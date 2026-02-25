CREATE TABLE status_hashtags (
    status_id  TEXT NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
    hashtag_id TEXT NOT NULL REFERENCES hashtags(id),
    PRIMARY KEY (status_id, hashtag_id)
);

-- Hashtag timeline query: find statuses with a given tag, paginated.
CREATE INDEX idx_status_hashtags_tag ON status_hashtags (hashtag_id);
