-- name: CreateClaim :one
INSERT INTO claims (org_id, user_id, device_id, subscription_id, claim_type, description, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, org_id, user_id, device_id, subscription_id, claim_type, description,
          status, amount_xof, filed_at, reviewed_at, settled_at, created_at, updated_at;

-- name: GetClaimByID :one
SELECT id, org_id, user_id, device_id, subscription_id, claim_type, description,
       status, amount_xof, filed_at, reviewed_at, settled_at, created_at, updated_at
FROM claims
WHERE id = $1;

-- name: ListClaimsByOrgAndUser :many
SELECT id, org_id, user_id, device_id, subscription_id, claim_type, description,
       status, amount_xof, filed_at, reviewed_at, settled_at, created_at, updated_at
FROM claims
WHERE org_id = $1 AND user_id = $2
ORDER BY filed_at DESC
LIMIT $3 OFFSET $4;

-- name: ListClaimsByOrg :many
SELECT id, org_id, user_id, device_id, subscription_id, claim_type, description,
       status, amount_xof, filed_at, reviewed_at, settled_at, created_at, updated_at
FROM claims
WHERE org_id = $1
ORDER BY filed_at DESC
LIMIT $2 OFFSET $3;

-- name: ListClaimsByOrgAndStatus :many
SELECT id, org_id, user_id, device_id, subscription_id, claim_type, description,
       status, amount_xof, filed_at, reviewed_at, settled_at, created_at, updated_at
FROM claims
WHERE org_id = $1 AND status = $2
ORDER BY filed_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdateClaimStatus :exec
UPDATE claims
SET status = $2, amount_xof = $3, reviewed_at = $4, settled_at = $5, updated_at = now()
WHERE id = $1;
