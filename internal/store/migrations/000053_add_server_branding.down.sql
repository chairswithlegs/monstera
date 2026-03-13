ALTER TABLE monstera_settings
  DROP COLUMN IF EXISTS server_name,
  DROP COLUMN IF EXISTS server_description,
  DROP COLUMN IF EXISTS server_rules;
