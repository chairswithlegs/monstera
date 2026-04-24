-- Complete the FK cascade set for statuses(id). Without these, deleting a
-- status that is referenced as a boost/quote/pin/conversation-anchor by
-- another row blocks with a foreign-key violation. This surfaces both when
-- an admin domain-block suspend purges remote statuses in bulk and when
-- a local account deletion cascades through the owner's statuses.
--
-- Design choices per FK:
--
--   statuses.reblog_of_id -> CASCADE
--     A boost is a pointer to the original; once the original is gone the
--     boost is meaningless. Matches Mastodon: deleting a status unboosts
--     everyone who boosted it.
--
--   statuses.quoted_status_id -> SET NULL
--     A quote post is authored by the quoter; its content stands on its own.
--     Losing the quoted target should orphan the reference, not destroy the
--     quoter's post. Mirrors in_reply_to_id behaviour (migration 000082).
--
--   account_pins.status_id -> CASCADE
--     A pin is a pure pointer; deleting the status leaves nothing to pin.
--
--   account_conversations.last_status_id -> SET NULL
--     The conversation thread persists even if the most recent message row
--     is gone. The UI treats NULL as "no last status yet".

ALTER TABLE statuses
    DROP CONSTRAINT statuses_reblog_of_id_fkey,
    ADD  CONSTRAINT statuses_reblog_of_id_fkey FOREIGN KEY (reblog_of_id)
        REFERENCES statuses(id) ON DELETE CASCADE;

ALTER TABLE statuses
    DROP CONSTRAINT statuses_quoted_status_id_fkey,
    ADD  CONSTRAINT statuses_quoted_status_id_fkey FOREIGN KEY (quoted_status_id)
        REFERENCES statuses(id) ON DELETE SET NULL;

ALTER TABLE account_pins
    DROP CONSTRAINT account_pins_status_id_fkey,
    ADD  CONSTRAINT account_pins_status_id_fkey FOREIGN KEY (status_id)
        REFERENCES statuses(id) ON DELETE CASCADE;

ALTER TABLE account_conversations
    DROP CONSTRAINT account_conversations_last_status_id_fkey,
    ADD  CONSTRAINT account_conversations_last_status_id_fkey FOREIGN KEY (last_status_id)
        REFERENCES statuses(id) ON DELETE SET NULL;
