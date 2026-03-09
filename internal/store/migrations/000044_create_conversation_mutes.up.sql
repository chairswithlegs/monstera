CREATE TABLE conversation_mutes (
    account_id       TEXT NOT NULL REFERENCES accounts(id),
    conversation_id  TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, conversation_id)
);

CREATE INDEX idx_conversation_mutes_account ON conversation_mutes (account_id);
