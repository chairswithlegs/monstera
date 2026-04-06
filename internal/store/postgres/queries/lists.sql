-- name: CreateList :one
INSERT INTO lists (id, account_id, title, replies_policy, exclusive)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetListByID :one
SELECT * FROM lists WHERE id = $1;

-- name: ListListsByAccount :many
SELECT * FROM lists WHERE account_id = $1 ORDER BY title ASC;

-- name: UpdateList :one
UPDATE lists SET title = $2, replies_policy = $3, exclusive = $4 WHERE id = $1 RETURNING *;

-- name: DeleteList :exec
DELETE FROM lists WHERE id = $1;

-- name: ListListAccountIDs :many
SELECT account_id FROM list_accounts WHERE list_id = $1 ORDER BY account_id;

-- name: AddAccountToList :exec
INSERT INTO list_accounts (list_id, account_id) VALUES ($1, $2) ON CONFLICT (list_id, account_id) DO NOTHING;

-- name: RemoveAccountFromList :exec
DELETE FROM list_accounts WHERE list_id = $1 AND account_id = $2;

-- name: GetListIDsByMemberAccountID :many
SELECT la.list_id FROM list_accounts la
INNER JOIN lists l ON l.id = la.list_id
WHERE la.account_id = $1
ORDER BY la.list_id;

-- name: GetListsByMemberAccountID :many
SELECT l.* FROM lists l
INNER JOIN list_accounts la ON la.list_id = l.id
WHERE la.account_id = $1
ORDER BY l.id;

-- name: GetListTimeline :many
SELECT s.* FROM statuses s
INNER JOIN list_accounts la ON la.account_id = s.account_id
WHERE la.list_id = $1
  AND s.deleted_at IS NULL
  AND ($2::text IS NULL OR s.id < $2)
ORDER BY s.id DESC
LIMIT $3;
