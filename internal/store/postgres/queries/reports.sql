-- name: CreateReport :one
INSERT INTO reports (id, account_id, target_id, status_ids, comment, category)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetReport :one
SELECT * FROM reports WHERE id = $1;

-- name: ListReports :many
SELECT * FROM reports
WHERE ($1::text = '' OR state = $1)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: AssignReport :exec
UPDATE reports SET assigned_to_id = $2 WHERE id = $1;

-- name: ResolveReport :exec
UPDATE reports SET state = 'resolved', action_taken = $2, resolved_at = NOW() WHERE id = $1;

-- name: CountReportsByState :one
SELECT COUNT(*) FROM reports WHERE state = $1;
