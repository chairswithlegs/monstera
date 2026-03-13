CREATE TABLE admin_actions (
    id                TEXT PRIMARY KEY,
    moderator_id      TEXT NOT NULL REFERENCES users(id),
    target_account_id TEXT REFERENCES accounts(id) ON DELETE SET NULL,  -- NULL for non-account actions; SET NULL preserves audit trail after hard delete
    action            TEXT NOT NULL,
    comment           TEXT,
    metadata          JSONB,                          -- contextual data: report_id, domain, old_role, new_role, etc.
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_actions_target ON admin_actions (target_account_id, created_at DESC)
    WHERE target_account_id IS NOT NULL;
CREATE INDEX idx_admin_actions_moderator ON admin_actions (moderator_id, created_at DESC);
CREATE INDEX idx_admin_actions_created ON admin_actions (created_at DESC);
