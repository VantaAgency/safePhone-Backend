-- name: CreateSubscription :one
INSERT INTO subscriptions (org_id, user_id, device_id, plan_id, status, billing_cycle, current_period_start, current_period_end)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, user_id, device_id, plan_id, status, billing_cycle,
          current_period_start, current_period_end, cancelled_at, created_at, updated_at;

-- name: GetSubscriptionByID :one
SELECT id, org_id, user_id, device_id, plan_id, status, billing_cycle,
       current_period_start, current_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE id = $1;

-- name: ListSubscriptionsByOrgAndUser :many
SELECT id, org_id, user_id, device_id, plan_id, status, billing_cycle,
       current_period_start, current_period_end, cancelled_at, created_at, updated_at
FROM subscriptions
WHERE org_id = $1 AND user_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdateSubscriptionStatus :exec
UPDATE subscriptions
SET status = $2, cancelled_at = $3, updated_at = now()
WHERE id = $1;
