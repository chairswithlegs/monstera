-- name: CreateServerFilter :one
INSERT INTO server_filters (id, phrase, scope, action)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetServerFilter :one
SELECT * FROM server_filters WHERE id = $1;

-- name: ListServerFilters :many
SELECT * FROM server_filters ORDER BY created_at DESC;

-- name: UpdateServerFilter :one
UPDATE server_filters SET phrase = $2, scope = $3, action = $4, whole_word = $5, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteServerFilter :exec
DELETE FROM server_filters WHERE id = $1;
