-- +migrate DisableTransactionForThisMigration
-- (INSERT … ON CONFLICT is idempotent; no transaction needed.)

INSERT INTO instance_settings (key, value) VALUES
    ('instance_name',        'Monstera'),
    ('instance_description', 'A Mastodon-compatible ActivityPub server'),
    ('registration_mode',    'approval'),
    ('contact_email',        ''),
    ('max_status_chars',     '500'),
    ('media_max_bytes',      '10485760'),
    ('rules_text',           '')
ON CONFLICT (key) DO NOTHING;
