ALTER TABLE accounts
    ADD COLUMN followers_count INT NOT NULL DEFAULT 0,
    ADD COLUMN following_count INT NOT NULL DEFAULT 0,
    ADD COLUMN statuses_count INT NOT NULL DEFAULT 0;
