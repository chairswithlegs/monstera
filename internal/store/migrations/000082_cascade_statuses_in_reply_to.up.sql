-- Complete the cascade set started by 000080_cascade_account_fks.
-- statuses.in_reply_to_id is the self-referential FK from a reply to its
-- parent status. Without ON DELETE SET NULL, deleting any status that has
-- been replied to fails with a foreign-key violation — which in turn blocks
-- account deletion whenever the account has a status that someone replied
-- to.
--
-- SET NULL mirrors Mastodon's long-standing schema (db/schema.rb
-- on_delete: :nullify, unchanged since 2017). A reply's content belongs to
-- its author, not to the parent's author; deleting a parent (or the
-- parent's author) should orphan the reply, not destroy it. The threading
-- traversal treats NULL parents as the root of a thread.

ALTER TABLE statuses
    DROP CONSTRAINT statuses_in_reply_to_id_fkey,
    ADD  CONSTRAINT statuses_in_reply_to_id_fkey FOREIGN KEY (in_reply_to_id)
        REFERENCES statuses(id) ON DELETE SET NULL;
