CREATE TABLE polls (
    id         TEXT PRIMARY KEY,
    status_id  TEXT NOT NULL UNIQUE REFERENCES statuses(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ,
    multiple   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_polls_status ON polls (status_id);

CREATE TABLE poll_options (
    id        TEXT PRIMARY KEY,
    poll_id   TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    title     TEXT NOT NULL,
    position  INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_poll_options_poll ON poll_options (poll_id);

CREATE TABLE poll_votes (
    id         TEXT PRIMARY KEY,
    poll_id    TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    option_id  TEXT NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (poll_id, account_id, option_id)
);

CREATE INDEX idx_poll_votes_poll ON poll_votes (poll_id);
CREATE INDEX idx_poll_votes_account ON poll_votes (account_id);
