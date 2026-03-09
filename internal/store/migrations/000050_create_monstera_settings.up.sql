CREATE TABLE monstera_settings (
    id                TEXT PRIMARY KEY DEFAULT 'default',
    registration_mode TEXT NOT NULL DEFAULT 'open' CHECK (registration_mode IN ('open', 'approval', 'invite', 'closed'))
);

INSERT INTO monstera_settings (id, registration_mode)
SELECT 'default', COALESCE((SELECT value FROM instance_settings WHERE key = 'registration_mode'), 'open')
ON CONFLICT (id) DO NOTHING;
