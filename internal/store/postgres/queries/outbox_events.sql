-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (id, event_type, aggregate_type, aggregate_id, payload, created_at)
VALUES ($1, $2, $3, $4, $5, now());

-- name: GetAndLockUnpublishedOutboxEvents :many
SELECT id, event_type, aggregate_type, aggregate_id, payload, created_at
FROM outbox_events
WHERE published_at IS NULL
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxEventsPublished :exec
UPDATE outbox_events
SET published_at = now()
WHERE id = ANY(@ids::text[]);

-- name: DeletePublishedOutboxEventsBefore :exec
DELETE FROM outbox_events
WHERE published_at IS NOT NULL
AND published_at < $1;
