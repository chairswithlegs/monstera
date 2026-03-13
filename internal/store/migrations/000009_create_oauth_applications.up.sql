CREATE TABLE oauth_applications (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    client_id     TEXT NOT NULL UNIQUE,
    client_secret TEXT NOT NULL,
    redirect_uris TEXT NOT NULL,              -- newline-separated list (Mastodon convention)
    scopes        TEXT NOT NULL DEFAULT 'read',
    website       TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
