-- name: CreateAnnouncement :one
INSERT INTO announcements (id, content, starts_at, ends_at, all_day, published_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateAnnouncement :exec
UPDATE announcements
SET content = $2, starts_at = $3, ends_at = $4, all_day = $5, published_at = $6, updated_at = NOW()
WHERE id = $1;

-- name: GetAnnouncementByID :one
SELECT * FROM announcements WHERE id = $1;

-- name: ListActiveAnnouncements :many
SELECT * FROM announcements
WHERE published_at <= NOW()
  AND (starts_at IS NULL OR starts_at <= NOW())
  AND (ends_at IS NULL OR ends_at > NOW())
ORDER BY published_at DESC;

-- name: ListAllAnnouncements :many
SELECT * FROM announcements
ORDER BY published_at DESC;
