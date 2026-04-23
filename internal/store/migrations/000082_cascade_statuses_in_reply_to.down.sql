ALTER TABLE statuses
    DROP CONSTRAINT statuses_in_reply_to_id_fkey,
    ADD  CONSTRAINT statuses_in_reply_to_id_fkey FOREIGN KEY (in_reply_to_id)
        REFERENCES statuses(id);
