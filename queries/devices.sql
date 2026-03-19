-- name: CreateDevice :one
INSERT INTO devices (org_id, user_id, brand, model, imei, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, org_id, user_id, brand, model, imei, status, created_at, updated_at, deleted_at;

-- name: GetDeviceByID :one
SELECT id, org_id, user_id, brand, model, imei, status, created_at, updated_at, deleted_at
FROM devices
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetDeviceByIMEI :one
SELECT id, org_id, user_id, brand, model, imei, status, created_at, updated_at, deleted_at
FROM devices
WHERE imei = $1 AND deleted_at IS NULL;

-- name: ListDevicesByOrgAndUser :many
SELECT id, org_id, user_id, brand, model, imei, status, created_at, updated_at, deleted_at
FROM devices
WHERE org_id = $1 AND user_id = $2 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdateDevice :exec
UPDATE devices
SET brand = $2, model = $3, status = $4, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteDevice :exec
UPDATE devices
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;
