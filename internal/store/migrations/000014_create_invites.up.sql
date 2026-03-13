CREATE TABLE invites (
    id         TEXT PRIMARY KEY,
    code       TEXT NOT NULL UNIQUE,
    created_by TEXT NOT NULL REFERENCES users(id),
    max_uses   INT,                           -- NULL = unlimited
    uses       INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
