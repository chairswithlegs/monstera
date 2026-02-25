CREATE TABLE statuses (
    id               TEXT PRIMARY KEY,
    uri              TEXT NOT NULL UNIQUE,       -- AP IRI
    account_id       TEXT NOT NULL REFERENCES accounts(id),
    text             TEXT,                       -- original Markdown/plain text
    content          TEXT,                       -- rendered HTML
    content_warning  TEXT,
    visibility       TEXT NOT NULL,              -- 'public'|'unlisted'|'private'|'direct'
    language         TEXT,
    in_reply_to_id   TEXT REFERENCES statuses(id),
    reblog_of_id     TEXT REFERENCES statuses(id),
    ap_id            TEXT NOT NULL UNIQUE,
    ap_raw           JSONB,
    sensitive        BOOLEAN NOT NULL DEFAULT FALSE,
    local            BOOLEAN NOT NULL DEFAULT TRUE,
    edited_at        TIMESTAMPTZ,               -- NULL if never edited
    replies_count    INT NOT NULL DEFAULT 0,
    reblogs_count    INT NOT NULL DEFAULT 0,
    favourites_count INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ                -- soft delete
);

-- Account timeline: paginated by id DESC.
CREATE INDEX idx_statuses_account ON statuses (account_id, id DESC)
    WHERE deleted_at IS NULL;

-- Local public timeline (used constantly; partial index keeps it tight).
CREATE INDEX idx_statuses_local_public ON statuses (id DESC)
    WHERE local = TRUE AND visibility = 'public' AND deleted_at IS NULL;

-- Federated public timeline (adds remote public posts).
CREATE INDEX idx_statuses_public ON statuses (id DESC)
    WHERE visibility = 'public' AND deleted_at IS NULL;

-- Thread context: find all replies to a given status.
CREATE INDEX idx_statuses_reply ON statuses (in_reply_to_id)
    WHERE in_reply_to_id IS NOT NULL AND deleted_at IS NULL;

-- Finding boosts of a given status (for unboost and reblogged_by queries).
CREATE INDEX idx_statuses_reblog ON statuses (reblog_of_id, account_id)
    WHERE reblog_of_id IS NOT NULL;

-- Now that statuses exists, add the FK constraint on media_attachments.status_id.
ALTER TABLE media_attachments
    ADD CONSTRAINT fk_media_status
    FOREIGN KEY (status_id) REFERENCES statuses(id) ON DELETE SET NULL;
