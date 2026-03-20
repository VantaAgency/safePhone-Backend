package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// DeviceRepository implements domain.DeviceRepository using pgxpool.
type DeviceRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewDeviceRepository creates a new device repository.
func NewDeviceRepository(pool *pgxpool.Pool) *DeviceRepository {
	return &DeviceRepository{pool: pool, timeout: 5 * time.Second}
}

// nullableIMEI returns nil when the IMEI is empty so the DB stores NULL instead of "".
func nullableIMEI(imei string) *string {
	if imei == "" {
		return nil
	}
	return &imei
}

// scanIMEI converts a nullable DB value back to a plain string (empty when NULL).
func scanIMEI(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func marshalDeviceMetadata(metadata domain.DeviceMetadata) []byte {
	payload, err := json.Marshal(metadata.Normalize())
	if err != nil {
		return []byte("{}")
	}
	return payload
}

func scanDeviceMetadata(payload []byte) domain.DeviceMetadata {
	if len(payload) == 0 {
		return domain.DeviceMetadata{}
	}

	var metadata domain.DeviceMetadata
	if err := json.Unmarshal(payload, &metadata); err != nil {
		return domain.DeviceMetadata{}
	}
	return metadata.Normalize()
}

// Create inserts a new device.
func (r *DeviceRepository) Create(ctx context.Context, d *domain.Device) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO devices (org_id, user_id, device_type, brand, model, metadata, imei, status)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
		RETURNING id, created_at, updated_at
	`, d.OrgID, d.UserID, d.DeviceType, d.Brand, d.Model, marshalDeviceMetadata(d.Metadata), nullableIMEI(d.IMEI), d.Status).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

// GetByID returns a device by ID (excluding soft-deleted).
func (r *DeviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Device, error) {
	if err := expireEndedSubscriptions(ctx, r.pool, r.timeout, nil, nil, nil, &id); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var d domain.Device
	var imei *string
	var metadata []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, device_type, brand, model, metadata, imei, status, created_at, updated_at, deleted_at
		FROM devices WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&d.ID, &d.OrgID, &d.UserID, &d.DeviceType, &d.Brand, &d.Model, &metadata, &imei, &d.Status, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	d.IMEI = scanIMEI(imei)
	d.Metadata = scanDeviceMetadata(metadata)
	return &d, err
}

// GetByIMEI returns a device by IMEI (excluding soft-deleted).
func (r *DeviceRepository) GetByIMEI(ctx context.Context, imei string) (*domain.Device, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var d domain.Device
	var scannedIMEI *string
	var metadata []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, device_type, brand, model, metadata, imei, status, created_at, updated_at, deleted_at
		FROM devices WHERE imei = $1 AND deleted_at IS NULL
	`, imei).Scan(&d.ID, &d.OrgID, &d.UserID, &d.DeviceType, &d.Brand, &d.Model, &metadata, &scannedIMEI, &d.Status, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	d.IMEI = scanIMEI(scannedIMEI)
	d.Metadata = scanDeviceMetadata(metadata)
	return &d, err
}

// ListByOrgAndUser returns devices for a specific org and user.
func (r *DeviceRepository) ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]domain.Device, error) {
	if err := expireEndedSubscriptions(ctx, r.pool, r.timeout, &orgID, &userID, nil, nil); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT id, org_id, user_id, device_type, brand, model, metadata, imei, status, created_at, updated_at, deleted_at
		FROM devices
		WHERE org_id = $1 AND user_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, orgID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		var d domain.Device
		var imei *string
		var metadata []byte
		if err := rows.Scan(&d.ID, &d.OrgID, &d.UserID, &d.DeviceType, &d.Brand, &d.Model, &metadata, &imei, &d.Status, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt); err != nil {
			return nil, err
		}
		d.IMEI = scanIMEI(imei)
		d.Metadata = scanDeviceMetadata(metadata)
		devices = append(devices, d)
	}

	if devices == nil {
		devices = []domain.Device{}
	}
	return devices, rows.Err()
}

// Update modifies a device.
func (r *DeviceRepository) Update(ctx context.Context, d *domain.Device) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE devices
		SET device_type = $2, brand = $3, model = $4, metadata = $5::jsonb, status = $6, imei = $7, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, d.ID, d.DeviceType, d.Brand, d.Model, marshalDeviceMetadata(d.Metadata), d.Status, nullableIMEI(d.IMEI))
	return err
}

// SoftDelete marks a device as deleted.
func (r *DeviceRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE devices SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}
