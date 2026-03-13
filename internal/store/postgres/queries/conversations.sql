-- name: CreateConversation :exec
INSERT INTO conversations (id, created_at) VALUES ($1, NOW());

-- name: SetStatusConversationID :exec
UPDATE statuses SET conversation_id = $2 WHERE id = $1;

-- name: GetStatusConversationID :one
SELECT conversation_id FROM statuses WHERE id = $1;

-- name: UpsertAccountConversation :exec
INSERT INTO account_conversations (id, account_id, conversation_id, last_status_id, unread, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (account_id, conversation_id) DO UPDATE SET
    last_status_id = EXCLUDED.last_status_id,
    unread = EXCLUDED.unread,
    updated_at = NOW();

-- name: ListAccountConversationsPaginated :many
SELECT * FROM account_conversations
WHERE account_id = $1
  AND ($2::text IS NULL OR id < $2)
ORDER BY updated_at DESC, id DESC
LIMIT $3;

-- name: GetAccountConversation :one
SELECT * FROM account_conversations WHERE account_id = $1 AND conversation_id = $2;

-- name: MarkAccountConversationRead :exec
UPDATE account_conversations SET unread = FALSE, updated_at = NOW()
WHERE account_id = $1 AND conversation_id = $2;

-- name: DeleteAccountConversation :exec
DELETE FROM account_conversations WHERE account_id = $1 AND conversation_id = $2;

-- name: GetConversationRoot :one
WITH RECURSIVE chain(depth, id, in_reply_to_id) AS (
    SELECT 1::bigint, statuses.id, statuses.in_reply_to_id FROM statuses WHERE statuses.id = $1
    UNION ALL
    SELECT c.depth + 1, s.id, s.in_reply_to_id FROM statuses s
    INNER JOIN chain c ON s.id = c.in_reply_to_id
    WHERE c.in_reply_to_id IS NOT NULL AND c.in_reply_to_id != '' AND c.depth < 1000
)
SELECT chain.id FROM chain
WHERE chain.in_reply_to_id IS NULL OR chain.in_reply_to_id = ''
ORDER BY chain.depth DESC
LIMIT 1;
