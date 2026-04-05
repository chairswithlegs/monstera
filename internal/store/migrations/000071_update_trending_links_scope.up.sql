ALTER TABLE monstera_settings
  DROP CONSTRAINT IF EXISTS monstera_settings_trending_links_scope_check;

ALTER TABLE monstera_settings
  ADD CONSTRAINT monstera_settings_trending_links_scope_check
    CHECK (trending_links_scope IN ('disabled', 'local', 'all'));

-- Migrate any existing 'users' rows to 'local'.
UPDATE monstera_settings SET trending_links_scope = 'local'
  WHERE trending_links_scope = 'users';
