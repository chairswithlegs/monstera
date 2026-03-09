-- name: CreateQuoteApproval :exec
INSERT INTO quote_approvals (quoting_status_id, quoted_status_id)
VALUES ($1, $2);

-- name: RevokeQuote :one
UPDATE quote_approvals
SET revoked_at = NOW()
WHERE quoting_status_id = $2 AND quoted_status_id = $1
RETURNING quoting_status_id;

-- name: GetQuoteApproval :one
SELECT quoting_status_id, quoted_status_id, revoked_at
FROM quote_approvals
WHERE quoting_status_id = $1;

-- name: ListQuotesOfStatus :many
SELECT s.* FROM statuses s
INNER JOIN quote_approvals qa ON qa.quoting_status_id = s.id
WHERE qa.quoted_status_id = $1 AND qa.revoked_at IS NULL AND s.deleted_at IS NULL
  AND ($2::text IS NULL OR s.id < $2)
ORDER BY s.id DESC
LIMIT $3;
