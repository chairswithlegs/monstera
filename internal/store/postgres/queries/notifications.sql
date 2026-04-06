-- name: CreateNotification :one
INSERT INTO notifications (id, account_id, from_id, type, status_id, group_key)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
WHERE account_id = $1
  AND ($2::text IS NULL OR id < $2)
  AND (cardinality($4::text[]) = 0 OR type = ANY($4::text[]))
  AND (cardinality($5::text[]) = 0 OR type != ALL($5::text[]))
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

-- name: ListGroupedNotifications :many
SELECT
  g.group_key,
  g.notifications_count,
  g.type,
  g.most_recent_notification_id,
  g.page_min_id,
  g.page_max_id,
  g.latest_page_notification_at,
  (SELECT ARRAY_AGG(sub.from_id)
   FROM (
     SELECT DISTINCT ON (n2.from_id) n2.from_id, n2.id
     FROM notifications n2
     WHERE n2.account_id = $1 AND n2.group_key = g.group_key
     ORDER BY n2.from_id, n2.id DESC
     LIMIT 8
   ) sub
  ) AS sample_account_ids,
  g.status_id
FROM (
  SELECT
    group_key,
    COUNT(*)::int AS notifications_count,
    MIN(type) AS type,
    MAX(id) AS most_recent_notification_id,
    MIN(id) AS page_min_id,
    MAX(id) AS page_max_id,
    MAX(created_at) AS latest_page_notification_at,
    (ARRAY_AGG(status_id ORDER BY id DESC))[1] AS status_id
  FROM notifications
  WHERE account_id = $1
    AND group_key != ''
    AND ($2::text IS NULL OR id < $2)
    AND (cardinality($4::text[]) = 0 OR type = ANY($4::text[]))
    AND (cardinality($5::text[]) = 0 OR type != ALL($5::text[]))
  GROUP BY group_key
  ORDER BY MAX(id) DESC
  LIMIT $3
) g;

-- name: GetNotificationGroup :many
SELECT * FROM notifications
WHERE account_id = $1 AND group_key = $2
ORDER BY id DESC;

-- name: DismissNotificationGroup :exec
DELETE FROM notifications WHERE account_id = $1 AND group_key = $2;

-- name: CountGroupedNotifications :one
SELECT COUNT(DISTINCT group_key)::bigint FROM notifications
WHERE account_id = $1 AND group_key != '' AND read = FALSE;
