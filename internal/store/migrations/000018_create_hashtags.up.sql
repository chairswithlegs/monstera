-- Interned hashtag strings. Names are stored lowercase-normalized.
-- Phase 2 will add usage counters / trending data here.
CREATE TABLE hashtags (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,          -- always lowercase (e.g. "golang")
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
