-- name: CreateEmailToken :one
INSERT INTO email_tokens (id, token_hash, user_id, purpose, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetEmailToken :one
SELECT * FROM email_tokens
WHERE token_hash = $1
  AND consumed_at IS NULL
  AND expires_at > NOW();

-- name: ConsumeEmailToken :exec
UPDATE email_tokens SET consumed_at = NOW()
WHERE token_hash = $1 AND consumed_at IS NULL;

-- name: DeleteExpiredEmailTokens :exec
DELETE FROM email_tokens WHERE expires_at < NOW();

-- name: DeleteEmailTokensForUser :exec
DELETE FROM email_tokens WHERE user_id = $1 AND consumed_at IS NULL;
