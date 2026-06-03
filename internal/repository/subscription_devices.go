package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// SubscriptionDevicesRepository owns the many-to-many table linking a
// subscription to every device it covers. Plans v2 introduced multi-device
// coverage (e.g. us_total covers 4 phones + 3 tablets + 2 PCs + 2 consoles
// + 1 TV) which the legacy single-FK subscriptions.device_id can't model.
//
// The legacy column still exists in lockstep — services that read it
// continue to function during the deprecation window. New code paths
// should call ListBySubscription / Attach from here.
type SubscriptionDevicesRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewSubscriptionDevicesRepository(pool *pgxpool.Pool) *SubscriptionDevicesRepository {
	return &SubscriptionDevicesRepository{pool: pool, timeout: 5 * time.Second}
}

// Attach inserts a subscription→device link. Idempotent: a duplicate
// attach returns nil without an error.
func (r *SubscriptionDevicesRepository) Attach(
	ctx context.Context,
	subscriptionID, deviceID uuid.UUID,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO subscription_devices (subscription_id, device_id, attached_at)
		VALUES ($1, $2, now())
		ON CONFLICT (subscription_id, device_id) DO NOTHING
	`, subscriptionID, deviceID)
	return err
}

// ListBySubscription returns every device attached to a subscription,
// ordered by attached_at ascending (oldest first — i.e. the device the
// user registered at sign-up comes first).
func (r *SubscriptionDevicesRepository) ListBySubscription(
	ctx context.Context,
	subscriptionID uuid.UUID,
) ([]domain.Device, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.org_id, d.user_id, d.device_type, d.brand, d.model, d.metadata, d.imei,
		       d.status, d.market,
		       d.verification_photos, d.verification_video, d.verification_status,
		       d.verified_at, d.verified_by, d.verification_rejected_reason,
		       d.created_at, d.updated_at, d.deleted_at
		FROM subscription_devices sd
		JOIN devices d ON d.id = sd.device_id
		WHERE sd.subscription_id = $1
		  AND d.deleted_at IS NULL
		ORDER BY sd.attached_at ASC
	`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		var d domain.Device
		var imei *string
		var metadata []byte
		if err := rows.Scan(
			&d.ID, &d.OrgID, &d.UserID, &d.DeviceType, &d.Brand, &d.Model, &metadata, &imei,
			&d.Status, &d.Market,
			&d.VerificationPhotos, &d.VerificationVideo, &d.VerificationStatus,
			&d.VerifiedAt, &d.VerifiedBy, &d.VerificationRejectedReason,
			&d.CreatedAt, &d.UpdatedAt, &d.DeletedAt,
		); err != nil {
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

// CountByType returns a tally of attached devices per type for a given
// subscription. Used by AttachDevices to enforce the plan's max_* caps
// before inserting a new row.
func (r *SubscriptionDevicesRepository) CountByType(
	ctx context.Context,
	subscriptionID uuid.UUID,
) (map[domain.DeviceType]int, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT d.device_type, COUNT(*)::int
		FROM subscription_devices sd
		JOIN devices d ON d.id = sd.device_id
		WHERE sd.subscription_id = $1
		  AND d.deleted_at IS NULL
		GROUP BY d.device_type
	`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[domain.DeviceType]int)
	for rows.Next() {
		var t domain.DeviceType
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, rows.Err()
}
