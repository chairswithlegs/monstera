-- name: CreateNotification :one
INSERT INTO notifications (id, account_id, from_id, type, status_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
WHERE account_id = $1
  AND ($2::text IS NULL OR id < $2)
ORDER BY id DESC
LIMIT $3;

-- name: GetNotification :one
SELECT * FROM notifications WHERE id = $1 AND account_id = $2;

-- name: DismissNotification :exec
DELETE FROM notifications WHERE id = $1 AND account_id = $2;

-- name: ClearNotifications :exec
DELETE FROM notifications WHERE account_id = $1;

-- name: MarkNotificationRead :exec
UPDATE notifications SET read = TRUE WHERE id = $1 AND account_id = $2;
