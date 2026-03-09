-- name: AddAnnouncementReaction :exec
INSERT INTO announcement_reactions (announcement_id, account_id, name)
VALUES ($1, $2, $3)
ON CONFLICT (announcement_id, account_id, name) DO NOTHING;

-- name: RemoveAnnouncementReaction :exec
DELETE FROM announcement_reactions
WHERE announcement_id = $1 AND account_id = $2 AND name = $3;

-- name: ListAnnouncementReactionCounts :many
SELECT name, COUNT(*)::bigint AS count
FROM announcement_reactions
WHERE announcement_id = $1
GROUP BY name;

-- name: ListAccountAnnouncementReactionNames :many
SELECT name FROM announcement_reactions
WHERE announcement_id = $1 AND account_id = $2;
