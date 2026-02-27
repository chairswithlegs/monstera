-- Admin-defined keyword/regex filters applied at content ingest.
CREATE TABLE server_filters (
    id         TEXT PRIMARY KEY,
    phrase     TEXT NOT NULL,                  -- literal keyword or regex pattern
    scope      TEXT NOT NULL DEFAULT 'all',    -- 'public_timeline'|'all'
    action     TEXT NOT NULL DEFAULT 'hide',   -- 'warn'|'hide'
    whole_word BOOLEAN NOT NULL DEFAULT FALSE,  -- match at word boundaries only (IMPLEMENTATION 10)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
