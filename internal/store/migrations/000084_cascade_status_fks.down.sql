-- Revert the cascade rules added in 000084. Restores the default NO ACTION
-- behaviour these columns had before the fixup.

ALTER TABLE statuses
    DROP CONSTRAINT statuses_reblog_of_id_fkey,
    ADD  CONSTRAINT statuses_reblog_of_id_fkey FOREIGN KEY (reblog_of_id)
        REFERENCES statuses(id);

ALTER TABLE statuses
    DROP CONSTRAINT statuses_quoted_status_id_fkey,
    ADD  CONSTRAINT statuses_quoted_status_id_fkey FOREIGN KEY (quoted_status_id)
        REFERENCES statuses(id);

ALTER TABLE account_pins
    DROP CONSTRAINT account_pins_status_id_fkey,
    ADD  CONSTRAINT account_pins_status_id_fkey FOREIGN KEY (status_id)
        REFERENCES statuses(id);

ALTER TABLE account_conversations
    DROP CONSTRAINT account_conversations_last_status_id_fkey,
    ADD  CONSTRAINT account_conversations_last_status_id_fkey FOREIGN KEY (last_status_id)
        REFERENCES statuses(id);
