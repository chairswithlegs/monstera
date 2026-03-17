CREATE TABLE push_subscriptions (
    id TEXT PRIMARY KEY,
    access_token_id TEXT NOT NULL REFERENCES oauth_access_tokens(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL,
    key_p256dh TEXT NOT NULL,
    key_auth TEXT NOT NULL,
    alerts JSONB NOT NULL DEFAULT '{}',
    policy TEXT NOT NULL DEFAULT 'all',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(access_token_id)
);

CREATE INDEX idx_push_subscriptions_account_id ON push_subscriptions(account_id);
