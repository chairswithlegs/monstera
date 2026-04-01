CREATE TABLE notification_policies (
    id                      TEXT        NOT NULL PRIMARY KEY,
    account_id              TEXT        NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    filter_not_following    TEXT        NOT NULL DEFAULT 'accept',
    filter_not_followers    TEXT        NOT NULL DEFAULT 'accept',
    filter_new_accounts     TEXT        NOT NULL DEFAULT 'accept',
    filter_private_mentions TEXT        NOT NULL DEFAULT 'accept',
    for_limited_accounts    TEXT        NOT NULL DEFAULT 'accept',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
