-- name: CreateAdminAction :one
INSERT INTO admin_actions (id, moderator_id, target_account_id, action, comment, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAdminActions :many
SELECT * FROM admin_actions
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListAdminActionsByTarget :many
SELECT * FROM admin_actions
WHERE target_account_id = $1
ORDER BY created_at DESC;

-- name: ListAdminActionsByModerator :many
SELECT * FROM admin_actions
WHERE moderator_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
