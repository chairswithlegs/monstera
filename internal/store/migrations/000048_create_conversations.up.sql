CREATE TABLE conversations (
    id         TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE statuses ADD COLUMN conversation_id TEXT REFERENCES conversations(id);
CREATE INDEX idx_statuses_conversation_id ON statuses (conversation_id)
    WHERE conversation_id IS NOT NULL;

CREATE TABLE account_conversations (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    last_status_id  TEXT REFERENCES statuses(id),
    unread          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, conversation_id)
);

CREATE INDEX idx_account_conversations_account
    ON account_conversations (account_id, updated_at DESC);
