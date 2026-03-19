-- name: CreatePayment :one
INSERT INTO payments (
  id, org_id, user_id, plan_id, subscription_id, amount_xof, currency,
  provider, payment_method, status, provider_ref, payment_url,
  idempotency_key, provider_payload, paid_at, failed_at, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING id, org_id, user_id, plan_id, subscription_id, amount_xof, currency,
          provider, payment_method, status, provider_ref, payment_url,
          idempotency_key, provider_payload, paid_at, failed_at, expires_at, created_at, updated_at;

-- name: GetPaymentByID :one
SELECT id, org_id, user_id, plan_id, subscription_id, amount_xof, currency,
       provider, payment_method, status, provider_ref, payment_url,
       idempotency_key, provider_payload, paid_at, failed_at, expires_at, created_at, updated_at
FROM payments
WHERE id = $1;

-- name: GetPaymentByIdempotencyKey :one
SELECT id, org_id, user_id, plan_id, subscription_id, amount_xof, currency,
       provider, payment_method, status, provider_ref, payment_url,
       idempotency_key, provider_payload, paid_at, failed_at, expires_at, created_at, updated_at
FROM payments
WHERE idempotency_key = $1;

-- name: ListPaymentsByOrgAndUser :many
SELECT id, org_id, user_id, plan_id, subscription_id, amount_xof, currency,
       provider, payment_method, status, provider_ref, payment_url,
       idempotency_key, provider_payload, paid_at, failed_at, expires_at, created_at, updated_at
FROM payments
WHERE org_id = $1 AND user_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdatePaymentStatus :exec
UPDATE payments
SET status = $2,
    payment_method = $3,
    provider_ref = $4,
    payment_url = $5,
    provider_payload = $6,
    paid_at = $7,
    failed_at = $8,
    expires_at = $9,
    updated_at = now()
WHERE id = $1;
