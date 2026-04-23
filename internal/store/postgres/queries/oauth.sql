-- name: CreateApplication :one
INSERT INTO oauth_applications (id, name, client_id, client_secret, redirect_uris, scopes, website)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetApplicationByClientID :one
SELECT * FROM oauth_applications WHERE client_id = $1;

-- name: GetApplicationByID :one
SELECT * FROM oauth_applications WHERE id = $1;

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

-- name: CreateAccessToken :one
INSERT INTO oauth_access_tokens (id, application_id, account_id, token, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAccessToken :one
SELECT * FROM oauth_access_tokens
WHERE token = $1 AND revoked_at IS NULL;

-- name: RevokeAccessToken :exec
UPDATE oauth_access_tokens SET revoked_at = NOW() WHERE token = $1;

-- name: ListAuthorizedApplicationsForAccount :many
SELECT DISTINCT ON (app.id)
    app.id AS application_id,
    app.name,
    app.website,
    app.redirect_uris,
    app.scopes AS app_scopes,
    token.scopes AS token_scopes,
    token.created_at AS authorized_at
FROM oauth_access_tokens token
JOIN oauth_applications app ON app.id = token.application_id
WHERE token.account_id = $1
  AND token.revoked_at IS NULL
  AND (token.expires_at IS NULL OR token.expires_at > NOW())
ORDER BY app.id, token.created_at DESC;

-- name: RevokeAccessTokensForAccountApp :many
UPDATE oauth_access_tokens
SET revoked_at = NOW()
WHERE account_id = $1
  AND application_id = $2
  AND revoked_at IS NULL
RETURNING token;
