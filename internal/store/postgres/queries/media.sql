-- name: CreateMediaAttachment :one
INSERT INTO media_attachments (id, account_id, type, content_type, storage_key, url, preview_url, remote_url, description, blurhash, meta)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetMediaAttachment :one
SELECT * FROM media_attachments WHERE id = $1;

-- name: UpdateMediaAttachment :one
UPDATE media_attachments SET
    description = $2,
    meta        = $3
WHERE id = $1 AND account_id = $4
RETURNING *;

-- name: AttachMediaToStatus :exec
UPDATE media_attachments SET status_id = $2
WHERE id = $1 AND account_id = $3;

-- name: ListStatusAttachments :many
SELECT * FROM media_attachments WHERE status_id = $1 ORDER BY id ASC;

-- name: ListUnattachedMedia :many
SELECT * FROM media_attachments
WHERE account_id = $1 AND status_id IS NULL
ORDER BY created_at DESC;
