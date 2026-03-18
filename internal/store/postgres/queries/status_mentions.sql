-- name: CreateStatusMention :exec
INSERT INTO status_mentions (status_id, account_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: GetStatusMentions :many
SELECT sqlc.embed(a), am.url AS avatar_url, hm.url AS header_url
FROM accounts a
INNER JOIN status_mentions sm ON sm.account_id = a.id
LEFT JOIN media_attachments am ON am.id = a.avatar_media_id
LEFT JOIN media_attachments hm ON hm.id = a.header_media_id
WHERE sm.status_id = $1;

-- name: GetStatusMentionAccountIDs :many
SELECT sm.account_id FROM status_mentions sm
INNER JOIN accounts a ON a.id = sm.account_id
WHERE sm.status_id = $1
  AND a.domain IS NULL;

-- name: DeleteStatusMentions :exec
DELETE FROM status_mentions WHERE status_id = $1;
