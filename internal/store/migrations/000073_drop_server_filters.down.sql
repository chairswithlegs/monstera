CREATE TABLE server_filters (
    id         TEXT PRIMARY KEY,
    phrase     TEXT NOT NULL,
    scope      TEXT NOT NULL DEFAULT 'all',
    action     TEXT NOT NULL DEFAULT 'hide',
    whole_word BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
