-- name: CreateConversationMute :exec
INSERT INTO conversation_mutes (account_id, conversation_id)
VALUES ($1, $2)
ON CONFLICT (account_id, conversation_id) DO NOTHING;

-- name: DeleteConversationMute :exec
DELETE FROM conversation_mutes
WHERE account_id = $1 AND conversation_id = $2;

-- name: IsConversationMuted :one
SELECT EXISTS(
    SELECT 1 FROM conversation_mutes
    WHERE account_id = $1 AND conversation_id = $2
);

-- name: ListMutedConversationIDs :many
SELECT conversation_id FROM conversation_mutes
WHERE account_id = $1;
