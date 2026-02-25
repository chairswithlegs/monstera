-- Short-lived codes bridging /oauth/authorize redirect and /oauth/token exchange.
-- Deleted on use; no replay is possible.
CREATE TABLE oauth_authorization_codes (
    id                     TEXT PRIMARY KEY,
    code                   TEXT NOT NULL UNIQUE,
    application_id         TEXT NOT NULL REFERENCES oauth_applications(id),
    account_id             TEXT NOT NULL REFERENCES accounts(id),
    redirect_uri           TEXT NOT NULL,
    scopes                 TEXT NOT NULL,
    code_challenge         TEXT,             -- PKCE: base64url(SHA-256(verifier))
    code_challenge_method  TEXT,             -- 'S256' only; NULL if no PKCE
    expires_at             TIMESTAMPTZ NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
