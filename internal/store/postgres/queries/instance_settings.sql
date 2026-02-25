-- name: GetSetting :one
SELECT value FROM instance_settings WHERE key = $1;

-- name: SetSetting :exec
INSERT INTO instance_settings (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = $2;

-- name: ListSettings :many
SELECT * FROM instance_settings ORDER BY key ASC;
