-- ─── Notification policy ─────────────────────────────────────────────────────

-- name: UpsertNotificationPolicy :one
INSERT INTO notification_policies (id, account_id)
VALUES ($1, $2)
ON CONFLICT (account_id) DO UPDATE SET id = notification_policies.id
RETURNING *;

-- name: GetNotificationPolicyByAccountID :one
SELECT * FROM notification_policies WHERE account_id = $1;

-- name: UpdateNotificationPolicy :one
UPDATE notification_policies
SET filter_not_following    = $2,
    filter_not_followers    = $3,
    filter_new_accounts     = $4,
    filter_private_mentions = $5,
    updated_at              = NOW()
WHERE account_id = $1
RETURNING *;

-- name: CountPendingNotificationRequests :one
SELECT COUNT(*) FROM notification_requests WHERE account_id = $1;

-- name: CountPendingNotifications :one
SELECT COALESCE(SUM(notifications_count), 0)::bigint FROM notification_requests WHERE account_id = $1;

-- ─── Notification requests ────────────────────────────────────────────────────

-- name: UpsertNotificationRequest :one
INSERT INTO notification_requests (id, account_id, from_account_id, last_status_id, notifications_count)
VALUES ($1, $2, $3, $4, 1)
ON CONFLICT (account_id, from_account_id) DO UPDATE
SET last_status_id      = EXCLUDED.last_status_id,
    notifications_count = notification_requests.notifications_count + 1,
    updated_at          = NOW()
RETURNING *;

-- name: GetNotificationRequestByID :one
SELECT * FROM notification_requests WHERE id = $1 AND account_id = $2;

-- name: ListNotificationRequests :many
SELECT * FROM notification_requests
WHERE account_id = $1
  AND ($2::text IS NULL OR id < $2)
ORDER BY id DESC
LIMIT $3;

-- name: DeleteNotificationRequest :exec
DELETE FROM notification_requests WHERE id = $1 AND account_id = $2;

-- name: DeleteNotificationRequestsByIDs :exec
DELETE FROM notification_requests WHERE account_id = $1 AND id = ANY($2::text[]);
