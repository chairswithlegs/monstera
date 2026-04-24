-- Distinguish two causes of a remote account being hidden:
--   * accounts.suspended        -- individual suspension (moderator action,
--                                  federation Delete{Person}). Only cleared
--                                  by a direct admin un-suspend.
--   * accounts.domain_suspended -- hidden because the admin has an active
--                                  severity=suspend domain block covering
--                                  the account's domain. Set atomically
--                                  with CreateDomainBlock; cleared by
--                                  DeleteDomainBlock.
--
-- Read paths that hide accounts (GET /accounts/:id, accounts/lookup,
-- delivery filters) must check BOTH columns. See account.IsHidden() helper.
-- Existing rows default to domain_suspended=FALSE so any account that was
-- already suspended pre-migration retains its current visibility and is
-- not affected by domain-block removals.

ALTER TABLE accounts
    ADD COLUMN domain_suspended BOOLEAN NOT NULL DEFAULT FALSE;
