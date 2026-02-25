-- name: CreateStatusMention :exec
INSERT INTO status_mentions (status_id, account_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetStatusMentions :many
SELECT a.* FROM accounts a
INNER JOIN status_mentions sm ON sm.account_id = a.id
WHERE sm.status_id = $1;
