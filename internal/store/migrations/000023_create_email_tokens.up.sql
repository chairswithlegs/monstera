CREATE TABLE email_tokens (
    id          TEXT PRIMARY KEY,
    token_hash  TEXT NOT NULL UNIQUE,            -- SHA-256 of the raw token
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     TEXT NOT NULL,                   -- 'confirmation'|'password_reset'
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,                     -- set on use; NULL until consumed
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Token lookup on confirmation/reset: find valid, unconsumed token.
CREATE INDEX idx_email_tokens_hash ON email_tokens (token_hash)
    WHERE consumed_at IS NULL;

-- Reaper query: delete expired tokens.
CREATE INDEX idx_email_tokens_expires ON email_tokens (expires_at)
    WHERE consumed_at IS NULL;
