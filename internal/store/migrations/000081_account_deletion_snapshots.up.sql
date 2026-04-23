-- Side tables that outlive the hard-delete of an account row so federation can
-- still deliver Delete{Actor} to remote followers after the CASCADE on
-- accounts(id) has dropped the follows/users/etc.
--
-- At delete time the service populates both tables inside the same tx that
-- deletes the account. The federation subscriber and the delivery worker read
-- from these tables instead of the (now-gone) accounts and follows rows.
-- A scheduler job purges snapshots past expires_at; CASCADE drops the targets
-- along with them.

CREATE TABLE account_deletion_snapshots (
    id              TEXT PRIMARY KEY,
    ap_id           TEXT NOT NULL,
    private_key_pem TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_account_deletion_snapshots_expires_at
    ON account_deletion_snapshots (expires_at);

CREATE TABLE account_deletion_targets (
    deletion_id  TEXT NOT NULL REFERENCES account_deletion_snapshots(id) ON DELETE CASCADE,
    inbox_url    TEXT NOT NULL,
    delivered_at TIMESTAMPTZ,
    PRIMARY KEY (deletion_id, inbox_url)
);

CREATE INDEX idx_account_deletion_targets_pending
    ON account_deletion_targets (deletion_id, inbox_url)
    WHERE delivered_at IS NULL;
