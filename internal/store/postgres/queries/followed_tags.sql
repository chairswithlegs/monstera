-- name: FollowTag :exec
INSERT INTO account_followed_tags (id, account_id, tag_id)
VALUES ($1, $2, $3)
ON CONFLICT (account_id, tag_id) DO NOTHING;

-- name: UnfollowTag :exec
DELETE FROM account_followed_tags
WHERE account_id = $1 AND tag_id = $2;

-- name: IsFollowingTag :one
SELECT EXISTS(
    SELECT 1 FROM account_followed_tags WHERE account_id = $1 AND tag_id = $2
) AS following;

-- name: ListFollowedTagsPaginated :many
SELECT aft.id AS cursor, h.id, h.name, h.created_at, h.updated_at
FROM account_followed_tags aft
INNER JOIN hashtags h ON h.id = aft.tag_id
WHERE aft.account_id = $1
  AND ($2::text IS NULL OR aft.id < $2)
ORDER BY aft.id DESC
LIMIT $3;
