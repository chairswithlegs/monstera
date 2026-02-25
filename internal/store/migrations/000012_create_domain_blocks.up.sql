CREATE TABLE domain_blocks (
    id         TEXT PRIMARY KEY,
    domain     TEXT NOT NULL UNIQUE,
    severity   TEXT NOT NULL DEFAULT 'suspend',  -- 'silence'|'suspend'
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
