-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByAccountID :one
SELECT * FROM users WHERE account_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (id, account_id, email, password_hash, role, registration_reason)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ConfirmUser :exec
UPDATE users SET confirmed_at = NOW() WHERE id = $1;

-- name: UpdateUserRole :exec
UPDATE users SET role = $2 WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: ListLocalUsers :many
SELECT u.* FROM users u
INNER JOIN accounts a ON a.id = u.account_id
WHERE a.domain IS NULL
ORDER BY u.created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetPendingRegistrations :many
SELECT u.* FROM users u
INNER JOIN accounts a ON a.id = u.account_id
WHERE u.confirmed_at IS NULL AND a.domain IS NULL
ORDER BY u.created_at ASC;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
