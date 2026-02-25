CREATE TABLE media_attachments (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts(id),
    status_id   TEXT,                           -- FK to statuses added in migration 000005
    type        TEXT NOT NULL,                  -- 'image'|'video'|'audio'|'gifv'
    storage_key TEXT NOT NULL,                  -- opaque key handed to MediaStore
    url         TEXT NOT NULL,                  -- public URL
    preview_url TEXT,
    remote_url  TEXT,                           -- original URL for remote media
    description TEXT,                           -- alt text
    blurhash    TEXT,
    meta        JSONB,                          -- width, height, duration, focal point, etc.
    size_bytes  BIGINT NOT NULL DEFAULT 0,      -- file size in bytes; set at upload time (ADR 10)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fetch all media for a given account (admin media panel, profile cleanup).
CREATE INDEX idx_media_account ON media_attachments (account_id);
-- Fetch all attachments belonging to a status (rendered in status response).
CREATE INDEX idx_media_status ON media_attachments (status_id) WHERE status_id IS NOT NULL;
