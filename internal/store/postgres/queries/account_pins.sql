-- name: CreateAccountPin :exec
INSERT INTO account_pins (account_id, status_id) VALUES ($1, $2)
ON CONFLICT (account_id, status_id) DO NOTHING;

-- name: DeleteAccountPin :exec
DELETE FROM account_pins WHERE account_id = $1 AND status_id = $2;

-- name: ListPinnedStatusIDs :many
SELECT status_id FROM account_pins
WHERE account_id = $1
ORDER BY created_at ASC;

-- name: DeleteAccountPinsByAccountID :exec
DELETE FROM account_pins WHERE account_id = $1;

-- name: CountAccountPins :one
SELECT COUNT(*) FROM account_pins WHERE account_id = $1;
