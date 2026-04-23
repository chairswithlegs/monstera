-- name: CreateAccountDeletionSnapshot :exec
INSERT INTO account_deletion_snapshots (id, ap_id, private_key_pem, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetAccountDeletionSnapshot :one
SELECT * FROM account_deletion_snapshots WHERE id = $1;

-- name: InsertAccountDeletionTargetsForAccount :exec
INSERT INTO account_deletion_targets (deletion_id, inbox_url)
SELECT DISTINCT $1, a.inbox_url
FROM follows f
INNER JOIN accounts a ON a.id = f.account_id
WHERE f.target_id = $2
  AND f.state = 'accepted'
  AND a.domain IS NOT NULL
  AND a.suspended = FALSE
  AND a.inbox_url <> ''
ON CONFLICT (deletion_id, inbox_url) DO NOTHING;

-- name: ListPendingAccountDeletionTargets :many
SELECT inbox_url
FROM account_deletion_targets
WHERE deletion_id = $1
  AND delivered_at IS NULL
  AND ($2::text IS NULL OR $2::text = '' OR inbox_url > $2)
ORDER BY inbox_url
LIMIT $3;

-- name: MarkAccountDeletionTargetDelivered :exec
UPDATE account_deletion_targets
SET delivered_at = NOW()
WHERE deletion_id = $1 AND inbox_url = $2;

-- name: DeleteExpiredAccountDeletionSnapshots :execrows
DELETE FROM account_deletion_snapshots
WHERE expires_at < $1;
