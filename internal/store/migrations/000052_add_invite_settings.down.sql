ALTER TABLE monstera_settings
  DROP COLUMN IF EXISTS invite_max_uses,
  DROP COLUMN IF EXISTS invite_expires_in_days;
