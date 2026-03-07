-- name: DismissAnnouncement :exec
INSERT INTO announcement_reads (account_id, announcement_id)
VALUES ($1, $2)
ON CONFLICT (account_id, announcement_id) DO NOTHING;

-- name: ListReadAnnouncementIDs :many
SELECT announcement_id FROM announcement_reads
WHERE account_id = $1;
