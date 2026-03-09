-- name: GetMonsteraSettings :one
SELECT id, registration_mode FROM monstera_settings WHERE id = 'default';

-- name: UpdateMonsteraSettings :exec
INSERT INTO monstera_settings (id, registration_mode) VALUES ('default', $1)
ON CONFLICT (id) DO UPDATE SET registration_mode = $1;
