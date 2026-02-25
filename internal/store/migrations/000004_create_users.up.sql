CREATE TABLE users (
    id                   TEXT PRIMARY KEY,
    account_id           TEXT NOT NULL UNIQUE REFERENCES accounts(id),
    email                TEXT NOT NULL UNIQUE,
    password_hash        TEXT NOT NULL,                -- bcrypt, cost >= 12
    confirmed_at         TIMESTAMPTZ,                  -- NULL until email is confirmed
    role                 TEXT NOT NULL DEFAULT 'user', -- 'user'|'moderator'|'admin'
    registration_reason TEXT,                          -- optional reason for registration (ADR 10)
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast lookup during login and admin queries.
CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_role ON users (role) WHERE role IN ('moderator', 'admin');
