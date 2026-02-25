CREATE TABLE follows (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id),  -- follower
    target_id  TEXT NOT NULL REFERENCES accounts(id),  -- followee
    state      TEXT NOT NULL DEFAULT 'pending',         -- 'pending'|'accepted'
    ap_id      TEXT UNIQUE,                             -- AP Follow activity IRI
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, target_id)
);

-- Find all followers of a given account (followers list, federation fan-out).
CREATE INDEX idx_follows_target ON follows (target_id, id DESC)
    WHERE state = 'accepted';

-- Find pending follow requests for approval queue.
CREATE INDEX idx_follows_pending ON follows (target_id)
    WHERE state = 'pending';
