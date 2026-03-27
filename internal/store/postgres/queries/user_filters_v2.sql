-- ─── Filter v2 CRUD ──────────────────────────────────────────────────────────

-- name: CreateUserFilterV2 :one
INSERT INTO user_filters (id, account_id, title, context, expires_at, filter_action, phrase, whole_word, irreversible)
VALUES ($1, $2, $3, $4, $5, $6, '', FALSE, FALSE)
RETURNING *;

-- name: GetUserFilterV2 :one
SELECT * FROM user_filters WHERE id = $1;

-- name: ListUserFiltersV2 :many
SELECT * FROM user_filters WHERE account_id = $1 ORDER BY created_at DESC;

-- name: UpdateUserFilterV2 :one
UPDATE user_filters
SET title = $2, context = $3, expires_at = $4, filter_action = $5
WHERE id = $1
RETURNING *;

-- name: DeleteUserFilterV2 :exec
DELETE FROM user_filters WHERE id = $1;

-- name: GetActiveUserFiltersV2 :many
SELECT * FROM user_filters
WHERE account_id = $1
  AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY created_at DESC;

-- ─── Filter keywords ─────────────────────────────────────────────────────────

-- name: CreateFilterKeyword :one
INSERT INTO user_filter_keywords (id, filter_id, keyword, whole_word)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetFilterKeyword :one
SELECT * FROM user_filter_keywords WHERE id = $1;

-- name: ListFilterKeywords :many
SELECT * FROM user_filter_keywords WHERE filter_id = $1 ORDER BY created_at;

-- name: UpdateFilterKeyword :one
UPDATE user_filter_keywords SET keyword = $2, whole_word = $3 WHERE id = $1 RETURNING *;

-- name: DeleteFilterKeyword :exec
DELETE FROM user_filter_keywords WHERE id = $1;

-- name: GetFilterKeywordsByFilterIDs :many
SELECT * FROM user_filter_keywords WHERE filter_id = ANY($1::text[]) ORDER BY filter_id, created_at;

-- ─── Filter statuses ─────────────────────────────────────────────────────────

-- name: CreateFilterStatus :one
INSERT INTO user_filter_statuses (id, filter_id, status_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetFilterStatus :one
SELECT * FROM user_filter_statuses WHERE id = $1;

-- name: ListFilterStatuses :many
SELECT * FROM user_filter_statuses WHERE filter_id = $1 ORDER BY created_at;

-- name: DeleteFilterStatus :exec
DELETE FROM user_filter_statuses WHERE id = $1;

-- name: GetFilterStatusesByFilterIDs :many
SELECT * FROM user_filter_statuses WHERE filter_id = ANY($1::text[]) ORDER BY filter_id, created_at;
