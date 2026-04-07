ALTER TABLE polls ADD COLUMN closed_at TIMESTAMPTZ;

UPDATE polls SET closed_at = expires_at
WHERE expires_at IS NOT NULL AND expires_at <= NOW();
