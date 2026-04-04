ALTER TABLE monstera_settings
  ADD COLUMN IF NOT EXISTS trending_tags_scope TEXT NOT NULL DEFAULT 'disabled'
    CHECK (trending_tags_scope IN ('disabled', 'local', 'all'));

ALTER TABLE monstera_settings
  ADD COLUMN IF NOT EXISTS trending_statuses_scope TEXT NOT NULL DEFAULT 'disabled'
    CHECK (trending_statuses_scope IN ('disabled', 'local', 'all'));
