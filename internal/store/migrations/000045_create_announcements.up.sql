CREATE TABLE announcements (
    id           TEXT PRIMARY KEY,
    content      TEXT NOT NULL,
    starts_at    TIMESTAMPTZ,
    ends_at      TIMESTAMPTZ,
    all_day      BOOLEAN NOT NULL DEFAULT false,
    published_at TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
