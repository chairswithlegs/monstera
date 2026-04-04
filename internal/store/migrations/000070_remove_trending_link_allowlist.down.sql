CREATE TABLE trending_link_allowlist (
    url        TEXT        NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
