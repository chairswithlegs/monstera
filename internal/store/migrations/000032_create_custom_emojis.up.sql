CREATE TABLE custom_emojis (
    id                TEXT PRIMARY KEY,
    shortcode         TEXT NOT NULL,               -- e.g. "blobcat" (without colons)
    domain            TEXT,                         -- NULL for local; domain for remote copies
    storage_key       TEXT,                         -- key in MediaStore (local emojis only)
    url               TEXT NOT NULL,                -- public URL of the emoji image
    static_url        TEXT,                         -- static (non-animated) version
    visible_in_picker BOOLEAN NOT NULL DEFAULT TRUE,
    disabled          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (shortcode, domain)
);

CREATE INDEX idx_custom_emojis_local ON custom_emojis (shortcode)
    WHERE domain IS NULL AND disabled = FALSE;
