-- name: CreateFavourite :one
INSERT INTO favourites (id, account_id, status_id, ap_id) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetFavouriteByAPID :one
SELECT * FROM favourites WHERE ap_id = $1;

-- name: GetFavouriteByAccountAndStatus :one
SELECT * FROM favourites WHERE account_id = $1 AND status_id = $2;

-- name: DeleteFavourite :exec
DELETE FROM favourites WHERE account_id = $1 AND status_id = $2;

-- name: IsFavourited :one
SELECT EXISTS(SELECT 1 FROM favourites WHERE account_id = $1 AND status_id = $2);

-- name: GetStatusFavouritedBy :many
SELECT a.* FROM accounts a
INNER JOIN favourites f ON f.account_id = a.id
WHERE f.status_id = $1
  AND ($2::text IS NULL OR f.id < $2)
ORDER BY f.id DESC
LIMIT $3;
