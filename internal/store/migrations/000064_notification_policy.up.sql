CREATE TABLE notification_policies (
    id                     TEXT        NOT NULL PRIMARY KEY,
    account_id             TEXT        NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    filter_not_following   BOOLEAN     NOT NULL DEFAULT FALSE,
    filter_not_followers   BOOLEAN     NOT NULL DEFAULT FALSE,
    filter_new_accounts    BOOLEAN     NOT NULL DEFAULT FALSE,
    filter_private_mentions BOOLEAN    NOT NULL DEFAULT FALSE,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_requests (
    id                  TEXT        NOT NULL PRIMARY KEY,
    account_id          TEXT        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    from_account_id     TEXT        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    last_status_id      TEXT        REFERENCES statuses(id) ON DELETE SET NULL,
    notifications_count INTEGER     NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, from_account_id)
);

CREATE INDEX notification_requests_account_id_idx ON notification_requests (account_id, id DESC);
