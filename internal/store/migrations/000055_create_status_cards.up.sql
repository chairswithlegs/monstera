CREATE TABLE status_cards (
    status_id        TEXT        NOT NULL PRIMARY KEY REFERENCES statuses(id) ON DELETE CASCADE,
    processing_state TEXT        NOT NULL DEFAULT 'no_url',
    url              TEXT        NOT NULL DEFAULT '',
    title            TEXT        NOT NULL DEFAULT '',
    description      TEXT        NOT NULL DEFAULT '',
    card_type        TEXT        NOT NULL DEFAULT 'link',
    provider_name    TEXT        NOT NULL DEFAULT '',
    provider_url     TEXT        NOT NULL DEFAULT '',
    image_url        TEXT        NOT NULL DEFAULT '',
    width            INTEGER     NOT NULL DEFAULT 0,
    height           INTEGER     NOT NULL DEFAULT 0,
    fetched_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
