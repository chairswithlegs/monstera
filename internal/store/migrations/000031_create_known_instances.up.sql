CREATE TABLE known_instances (
    id               TEXT PRIMARY KEY,
    domain           TEXT NOT NULL UNIQUE,
    software         TEXT,                         -- from NodeInfo: "mastodon", "pleroma", etc.
    software_version TEXT,
    first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_known_instances_last_seen ON known_instances (last_seen_at DESC);
