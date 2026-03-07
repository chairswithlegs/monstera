-- name: CreateUserFilter :one
INSERT INTO user_filters (id, account_id, phrase, context, whole_word, expires_at, irreversible)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUserFilter :one
SELECT * FROM user_filters WHERE id = $1;

-- name: ListUserFilters :many
SELECT * FROM user_filters WHERE account_id = $1 ORDER BY created_at DESC;

-- name: UpdateUserFilter :one
UPDATE user_filters SET phrase = $2, context = $3, whole_word = $4, expires_at = $5, irreversible = $6
WHERE id = $1 RETURNING *;

-- name: DeleteUserFilter :exec
DELETE FROM user_filters WHERE id = $1;

-- name: GetActiveUserFiltersByContext :many
SELECT * FROM user_filters
WHERE account_id = $1
  AND $2::text = ANY(context)
  AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY created_at DESC;
