-- name: CreateApplication :one
INSERT INTO oauth_applications (id, name, client_id, client_secret, redirect_uris, scopes, website)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetApplicationByClientID :one
SELECT * FROM oauth_applications WHERE client_id = $1;

-- name: CreateAuthorizationCode :one
INSERT INTO oauth_authorization_codes (
    id, code, application_id, account_id,
    redirect_uri, scopes, code_challenge, code_challenge_method, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetAuthorizationCode :one
SELECT * FROM oauth_authorization_codes
WHERE code = $1 AND expires_at > NOW();

-- name: DeleteAuthorizationCode :exec
DELETE FROM oauth_authorization_codes WHERE code = $1;

-- name: DeleteAuthorizationCodesForAccount :exec
DELETE FROM oauth_authorization_codes WHERE account_id = $1;

-- name: CreateAccessToken :one
INSERT INTO oauth_access_tokens (id, application_id, account_id, token, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAccessToken :one
SELECT * FROM oauth_access_tokens
WHERE token = $1 AND revoked_at IS NULL;

-- name: RevokeAccessToken :exec
UPDATE oauth_access_tokens SET revoked_at = NOW() WHERE token = $1;

-- name: RevokeAllAccessTokensForAccount :exec
UPDATE oauth_access_tokens SET revoked_at = NOW()
WHERE account_id = $1 AND revoked_at IS NULL;

-- Lists every access-token string ever issued to an account (including
-- already-revoked ones). Used by the OAuth cache invalidator after bulk
-- revocation so cached entries can be deleted by hash.
-- name: ListAccessTokenStringsForAccount :many
SELECT token FROM oauth_access_tokens WHERE account_id = $1;
