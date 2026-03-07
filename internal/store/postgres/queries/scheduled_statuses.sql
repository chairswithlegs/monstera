-- name: CreateScheduledStatus :one
INSERT INTO scheduled_statuses (id, account_id, params, scheduled_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetScheduledStatusByID :one
SELECT * FROM scheduled_statuses WHERE id = $1;

-- name: ListScheduledStatuses :many
SELECT * FROM scheduled_statuses
WHERE account_id = $1
  AND ($2::text IS NULL OR id < $2)
ORDER BY id DESC
LIMIT $3;

-- name: UpdateScheduledStatus :one
UPDATE scheduled_statuses
SET params = $2, scheduled_at = $3
WHERE id = $1
RETURNING *;

-- name: DeleteScheduledStatus :exec
DELETE FROM scheduled_statuses WHERE id = $1;

-- name: ListScheduledStatusesDue :many
SELECT * FROM scheduled_statuses
WHERE scheduled_at <= NOW()
ORDER BY scheduled_at ASC
LIMIT $1;
