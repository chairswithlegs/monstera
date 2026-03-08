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
