CREATE TABLE accounts (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL,
    domain        TEXT,                         -- NULL for local accounts
    display_name  TEXT,
    note          TEXT,                         -- bio (HTML)
    -- avatar_media_id and header_media_id added in migration 000003
    public_key    TEXT NOT NULL,                -- RSA-2048 public key (PEM)
    private_key   TEXT,                         -- NULL for remote accounts; encrypted at rest
    inbox_url     TEXT NOT NULL,
    outbox_url    TEXT NOT NULL,
    followers_url TEXT NOT NULL,
    following_url TEXT NOT NULL,
    ap_id         TEXT NOT NULL UNIQUE,         -- canonical ActivityPub IRI
    ap_raw        JSONB,                        -- raw AP Actor document
    bot           BOOLEAN NOT NULL DEFAULT FALSE,
    locked        BOOLEAN NOT NULL DEFAULT FALSE,  -- requires follow approval
    suspended     BOOLEAN NOT NULL DEFAULT FALSE,
    silenced      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (username, domain)
);

-- Used by WebFinger lookups (local accounts: domain IS NULL).
CREATE INDEX idx_accounts_username_local ON accounts (username) WHERE domain IS NULL;
-- Used when refreshing remote actor profiles.
CREATE INDEX idx_accounts_domain ON accounts (domain) WHERE domain IS NOT NULL;
