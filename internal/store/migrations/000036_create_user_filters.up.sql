CREATE TABLE user_filters (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    phrase      TEXT NOT NULL,
    context     TEXT[] NOT NULL DEFAULT '{}',
    whole_word  BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at  TIMESTAMPTZ,
    irreversible BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_filters_account ON user_filters (account_id);
