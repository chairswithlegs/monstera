-- name: CreateStatusMention :exec
INSERT INTO status_mentions (status_id, account_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetStatusMentions :many
SELECT a.* FROM accounts a
INNER JOIN status_mentions sm ON sm.account_id = a.id
WHERE sm.status_id = $1;

-- name: GetStatusMentionAccountIDs :many
SELECT sm.account_id FROM status_mentions sm
INNER JOIN accounts a ON a.id = sm.account_id
WHERE sm.status_id = $1
  AND a.domain IS NULL;

-- name: DeleteStatusMentions :exec
DELETE FROM status_mentions WHERE status_id = $1;
