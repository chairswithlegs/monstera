ALTER TABLE poll_options ADD COLUMN votes_count INT NOT NULL DEFAULT 0;

UPDATE poll_options SET votes_count = (
    SELECT COUNT(*) FROM poll_votes WHERE poll_votes.option_id = poll_options.id
);
