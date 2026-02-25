-- name: CreateCustomEmoji :one
INSERT INTO custom_emojis (id, shortcode, domain, storage_key, url, static_url, visible_in_picker)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListLocalCustomEmojis :many
SELECT * FROM custom_emojis
WHERE domain IS NULL AND disabled = FALSE
ORDER BY shortcode ASC;

-- name: ListAllCustomEmojis :many
SELECT * FROM custom_emojis
ORDER BY domain NULLS FIRST, shortcode ASC
LIMIT $1 OFFSET $2;

-- name: GetCustomEmojiByShortcode :one
SELECT * FROM custom_emojis WHERE shortcode = $1 AND domain IS NULL;

-- name: DeleteCustomEmoji :exec
DELETE FROM custom_emojis WHERE shortcode = $1 AND domain IS NULL;
