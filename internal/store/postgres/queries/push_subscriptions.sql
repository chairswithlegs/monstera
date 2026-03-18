-- name: CreatePushSubscription :one
INSERT INTO push_subscriptions (id, access_token_id, account_id, endpoint, key_p256dh, key_auth, alerts, policy)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (access_token_id) DO UPDATE SET
    endpoint = EXCLUDED.endpoint,
    key_p256dh = EXCLUDED.key_p256dh,
    key_auth = EXCLUDED.key_auth,
    alerts = EXCLUDED.alerts,
    policy = EXCLUDED.policy,
    updated_at = now()
RETURNING id, access_token_id, account_id, endpoint, key_p256dh, key_auth, alerts, policy, created_at, updated_at;

-- name: GetPushSubscriptionByAccessToken :one
SELECT id, access_token_id, account_id, endpoint, key_p256dh, key_auth, alerts, policy, created_at, updated_at
FROM push_subscriptions
WHERE access_token_id = $1;

-- name: UpdatePushSubscriptionAlerts :one
UPDATE push_subscriptions
SET alerts = $2, policy = $3, updated_at = now()
WHERE access_token_id = $1
RETURNING id, access_token_id, account_id, endpoint, key_p256dh, key_auth, alerts, policy, created_at, updated_at;

-- name: DeletePushSubscription :exec
DELETE FROM push_subscriptions WHERE access_token_id = $1;

-- name: ListPushSubscriptionsByAccountID :many
SELECT id, access_token_id, account_id, endpoint, key_p256dh, key_auth, alerts, policy, created_at, updated_at
FROM push_subscriptions
WHERE account_id = $1;
