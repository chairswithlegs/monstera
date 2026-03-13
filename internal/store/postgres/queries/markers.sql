-- name: GetMarkers :many
SELECT account_id, timeline, last_read_id, version, updated_at
FROM markers
WHERE account_id = $1
  AND timeline = ANY($2::text[]);

-- name: SetMarker :exec
INSERT INTO markers (account_id, timeline, last_read_id, version, updated_at)
VALUES ($1, $2, $3, 0, NOW())
ON CONFLICT (account_id, timeline)
DO UPDATE SET
    last_read_id = EXCLUDED.last_read_id,
    version      = markers.version + 1,
    updated_at   = NOW();
