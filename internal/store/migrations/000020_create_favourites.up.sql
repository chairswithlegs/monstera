CREATE TABLE favourites (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    status_id  TEXT NOT NULL REFERENCES statuses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, status_id)
);

-- Phase 2 /api/v1/favourites: paginated list of a user's favourited posts.
CREATE INDEX idx_favourites_account ON favourites (account_id, id DESC);
-- /api/v1/statuses/:id/favourited_by: who favourited a specific status.
CREATE INDEX idx_favourites_status ON favourites (status_id);
