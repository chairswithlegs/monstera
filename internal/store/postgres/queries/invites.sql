-- name: CreateInvite :one
INSERT INTO invites (id, code, created_by, max_uses, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetInviteByCode :one
SELECT * FROM invites WHERE code = $1;

-- name: IncrementInviteUses :exec
UPDATE invites SET uses = uses + 1 WHERE code = $1;

-- name: ListInvitesByCreator :many
SELECT * FROM invites WHERE created_by = $1 ORDER BY created_at DESC;

-- name: DeleteInvite :exec
DELETE FROM invites WHERE id = $1;
