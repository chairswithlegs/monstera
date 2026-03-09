-- name: GetMonsteraSettings :one
SELECT id, registration_mode, invite_max_uses, invite_expires_in_days
FROM monstera_settings WHERE id = 'default';

-- name: UpdateMonsteraSettings :exec
INSERT INTO monstera_settings (id, registration_mode, invite_max_uses, invite_expires_in_days)
VALUES ('default', @registration_mode, @invite_max_uses, @invite_expires_in_days)
ON CONFLICT (id) DO UPDATE SET
  registration_mode = @registration_mode,
  invite_max_uses = @invite_max_uses,
  invite_expires_in_days = @invite_expires_in_days;
