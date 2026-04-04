ALTER TABLE monstera_settings
  ADD COLUMN IF NOT EXISTS trending_links_scope TEXT NOT NULL DEFAULT 'disabled'
    CHECK (trending_links_scope IN ('disabled', 'users', 'all'));
