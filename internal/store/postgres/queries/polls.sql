-- name: CreatePoll :one
INSERT INTO polls (id, status_id, expires_at, multiple)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreatePollOption :one
INSERT INTO poll_options (id, poll_id, title, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPollByID :one
SELECT * FROM polls WHERE id = $1;

-- name: GetPollByStatusID :one
SELECT * FROM polls WHERE status_id = $1;

-- name: ListPollOptions :many
SELECT * FROM poll_options WHERE poll_id = $1 ORDER BY position ASC, id ASC;

-- name: DeletePollVotesByAccount :exec
DELETE FROM poll_votes WHERE poll_id = $1 AND account_id = $2;

-- name: CreatePollVote :one
INSERT INTO poll_votes (id, poll_id, account_id, option_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: SetPollOptionVoteCount :exec
UPDATE poll_options SET votes_count = $3 WHERE poll_id = $1 AND position = $2;

-- name: ListExpiredOpenPollStatusIDs :many
SELECT p.status_id FROM polls p
WHERE p.expires_at IS NOT NULL AND p.expires_at <= NOW() AND p.closed_at IS NULL
ORDER BY p.expires_at ASC
LIMIT $1;

-- name: ClosePoll :exec
UPDATE polls SET closed_at = NOW() WHERE id = $1;

-- name: GetVoteCountsByPoll :many
SELECT option_id, COUNT(*)::int AS votes_count
FROM poll_votes
WHERE poll_id = $1
GROUP BY option_id;

-- name: CountDistinctVoters :one
SELECT COUNT(DISTINCT account_id)::int AS voters_count
FROM poll_votes WHERE poll_id = $1;

-- name: HasVotedOnPoll :one
SELECT EXISTS(
  SELECT 1 FROM poll_votes WHERE poll_id = $1 AND account_id = $2
) AS voted;

-- name: GetOwnVoteOptionIDs :many
SELECT option_id FROM poll_votes
WHERE poll_id = $1 AND account_id = $2
ORDER BY created_at ASC;
