ALTER TABLE users
    DROP CONSTRAINT users_account_id_fkey,
    ADD  CONSTRAINT users_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE media_attachments
    DROP CONSTRAINT media_attachments_account_id_fkey,
    ADD  CONSTRAINT media_attachments_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE statuses
    DROP CONSTRAINT statuses_account_id_fkey,
    ADD  CONSTRAINT statuses_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE statuses
    DROP CONSTRAINT statuses_in_reply_to_account_id_fkey,
    ADD  CONSTRAINT statuses_in_reply_to_account_id_fkey FOREIGN KEY (in_reply_to_account_id)
        REFERENCES accounts(id);

ALTER TABLE status_edits
    DROP CONSTRAINT status_edits_account_id_fkey,
    ADD  CONSTRAINT status_edits_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE follows
    DROP CONSTRAINT follows_account_id_fkey,
    ADD  CONSTRAINT follows_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE follows
    DROP CONSTRAINT follows_target_id_fkey,
    ADD  CONSTRAINT follows_target_id_fkey FOREIGN KEY (target_id)
        REFERENCES accounts(id);

ALTER TABLE notifications
    DROP CONSTRAINT notifications_account_id_fkey,
    ADD  CONSTRAINT notifications_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE notifications
    DROP CONSTRAINT notifications_from_id_fkey,
    ADD  CONSTRAINT notifications_from_id_fkey FOREIGN KEY (from_id)
        REFERENCES accounts(id);

ALTER TABLE oauth_access_tokens
    DROP CONSTRAINT oauth_access_tokens_account_id_fkey,
    ADD  CONSTRAINT oauth_access_tokens_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE oauth_authorization_codes
    DROP CONSTRAINT oauth_authorization_codes_account_id_fkey,
    ADD  CONSTRAINT oauth_authorization_codes_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

-- Restore reports FK to default (NO ACTION) and restore NOT NULL. Any rows
-- with NULL account_id / target_id (from SET NULL cascades while the up
-- migration was live) will block this DOWN — resolve by deleting those rows
-- first, which is the correct behavior for orphaned reports.
ALTER TABLE reports
    DROP CONSTRAINT reports_account_id_fkey,
    ADD  CONSTRAINT reports_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE reports
    DROP CONSTRAINT reports_target_id_fkey,
    ADD  CONSTRAINT reports_target_id_fkey FOREIGN KEY (target_id)
        REFERENCES accounts(id);

ALTER TABLE reports ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE reports ALTER COLUMN target_id  SET NOT NULL;

ALTER TABLE mutes
    DROP CONSTRAINT mutes_account_id_fkey,
    ADD  CONSTRAINT mutes_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE mutes
    DROP CONSTRAINT mutes_target_id_fkey,
    ADD  CONSTRAINT mutes_target_id_fkey FOREIGN KEY (target_id)
        REFERENCES accounts(id);

ALTER TABLE blocks
    DROP CONSTRAINT blocks_account_id_fkey,
    ADD  CONSTRAINT blocks_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE blocks
    DROP CONSTRAINT blocks_target_id_fkey,
    ADD  CONSTRAINT blocks_target_id_fkey FOREIGN KEY (target_id)
        REFERENCES accounts(id);

ALTER TABLE favourites
    DROP CONSTRAINT favourites_account_id_fkey,
    ADD  CONSTRAINT favourites_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE status_mentions
    DROP CONSTRAINT status_mentions_account_id_fkey,
    ADD  CONSTRAINT status_mentions_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE bookmarks
    DROP CONSTRAINT bookmarks_account_id_fkey,
    ADD  CONSTRAINT bookmarks_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE markers
    DROP CONSTRAINT markers_account_id_fkey,
    ADD  CONSTRAINT markers_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE account_pins
    DROP CONSTRAINT account_pins_account_id_fkey,
    ADD  CONSTRAINT account_pins_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE account_followed_tags
    DROP CONSTRAINT account_followed_tags_account_id_fkey,
    ADD  CONSTRAINT account_followed_tags_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE account_featured_tags
    DROP CONSTRAINT account_featured_tags_account_id_fkey,
    ADD  CONSTRAINT account_featured_tags_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE conversation_mutes
    DROP CONSTRAINT conversation_mutes_account_id_fkey,
    ADD  CONSTRAINT conversation_mutes_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE announcement_reads
    DROP CONSTRAINT announcement_reads_account_id_fkey,
    ADD  CONSTRAINT announcement_reads_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE announcement_reactions
    DROP CONSTRAINT announcement_reactions_account_id_fkey,
    ADD  CONSTRAINT announcement_reactions_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);

ALTER TABLE account_conversations
    DROP CONSTRAINT account_conversations_account_id_fkey,
    ADD  CONSTRAINT account_conversations_account_id_fkey FOREIGN KEY (account_id)
        REFERENCES accounts(id);
