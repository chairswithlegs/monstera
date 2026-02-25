-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByAccountID :one
SELECT * FROM users WHERE account_id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (id, account_id, email, password_hash, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ConfirmUser :exec
UPDATE users SET confirmed_at = NOW() WHERE id = $1;

-- name: UpdateUserRole :exec
UPDATE users SET role = $2 WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;
