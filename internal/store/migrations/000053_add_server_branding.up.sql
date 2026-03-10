ALTER TABLE monstera_settings
  ADD COLUMN IF NOT EXISTS server_name TEXT,
  ADD COLUMN IF NOT EXISTS server_description TEXT,
  ADD COLUMN IF NOT EXISTS server_rules TEXT;
