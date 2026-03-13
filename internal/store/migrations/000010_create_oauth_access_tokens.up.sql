CREATE TABLE oauth_access_tokens (
    id             TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES oauth_applications(id),
    account_id     TEXT REFERENCES accounts(id),  -- NULL for app-level tokens
    token          TEXT NOT NULL UNIQUE,
    scopes         TEXT NOT NULL,
    expires_at     TIMESTAMPTZ,               -- NULL = non-expiring (Mastodon default)
    revoked_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Revoke all tokens for a suspended/deleted account.
CREATE INDEX idx_oauth_tokens_account ON oauth_access_tokens (account_id)
    WHERE account_id IS NOT NULL;
