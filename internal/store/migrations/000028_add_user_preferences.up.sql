ALTER TABLE users
    ADD COLUMN default_privacy TEXT NOT NULL DEFAULT 'public',
    ADD COLUMN default_sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN default_language TEXT NOT NULL DEFAULT '';
