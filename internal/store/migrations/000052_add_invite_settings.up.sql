ALTER TABLE monstera_settings
  ADD COLUMN IF NOT EXISTS invite_max_uses INTEGER,
  ADD COLUMN IF NOT EXISTS invite_expires_in_days INTEGER;
