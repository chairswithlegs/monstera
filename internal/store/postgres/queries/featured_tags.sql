-- name: CreateFeaturedTag :exec
INSERT INTO account_featured_tags (id, account_id, tag_id)
VALUES ($1, $2, $3)
ON CONFLICT (account_id, tag_id) DO NOTHING;

-- name: DeleteFeaturedTag :exec
DELETE FROM account_featured_tags
WHERE id = $1 AND account_id = $2;

-- name: ListFeaturedTagsByAccount :many
SELECT aft.id,
       aft.account_id,
       aft.tag_id,
       h.name,
       aft.created_at,
       (SELECT COUNT(*)::bigint
        FROM status_hashtags sh
        INNER JOIN statuses st ON st.id = sh.status_id
        WHERE st.account_id = aft.account_id
          AND sh.hashtag_id = aft.tag_id
          AND st.deleted_at IS NULL) AS statuses_count,
       (SELECT MAX(st.created_at)
        FROM status_hashtags sh
        INNER JOIN statuses st ON st.id = sh.status_id
        WHERE st.account_id = aft.account_id
          AND sh.hashtag_id = aft.tag_id
          AND st.deleted_at IS NULL) AS last_status_at
FROM account_featured_tags aft
INNER JOIN hashtags h ON h.id = aft.tag_id
WHERE aft.account_id = $1
ORDER BY aft.created_at DESC;

-- name: GetFeaturedTagByID :one
SELECT aft.id, aft.account_id, aft.tag_id, h.name, aft.created_at
FROM account_featured_tags aft
INNER JOIN hashtags h ON h.id = aft.tag_id
WHERE aft.id = $1 AND aft.account_id = $2;

-- name: ListAccountTagSuggestions :many
SELECT h.id, h.name, COUNT(*)::bigint AS use_count
FROM status_hashtags sh
INNER JOIN hashtags h ON h.id = sh.hashtag_id
INNER JOIN statuses s ON s.id = sh.status_id
WHERE s.account_id = $1
  AND s.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM account_featured_tags aft WHERE aft.account_id = s.account_id AND aft.tag_id = h.id)
GROUP BY h.id, h.name
ORDER BY use_count DESC
LIMIT $2;
