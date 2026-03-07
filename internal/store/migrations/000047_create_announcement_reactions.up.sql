CREATE TABLE announcement_reactions (
    announcement_id TEXT NOT NULL REFERENCES announcements(id),
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    name            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (announcement_id, account_id, name)
);

CREATE INDEX idx_announcement_reactions_announcement ON announcement_reactions (announcement_id);
