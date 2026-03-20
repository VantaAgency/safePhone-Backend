package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

func expireEndedSubscriptions(
	ctx context.Context,
	pool *pgxpool.Pool,
	timeout time.Duration,
	orgID, userID, subscriptionID, deviceID *uuid.UUID,
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := pool.Exec(ctx, `
		WITH expired_subscriptions AS (
			UPDATE subscriptions AS s
			SET status = $5, updated_at = now()
			WHERE s.status = $6
			  AND s.current_period_end IS NOT NULL
			  AND s.current_period_end <= now()
			  AND ($1::uuid IS NULL OR s.org_id = $1)
			  AND ($2::uuid IS NULL OR s.user_id = $2)
			  AND ($3::uuid IS NULL OR s.id = $3)
			  AND ($4::uuid IS NULL OR s.device_id = $4)
			RETURNING s.device_id
		)
		UPDATE devices AS d
		SET status = $7, updated_at = now()
		WHERE d.deleted_at IS NULL
		  AND d.status NOT IN ($7, $8)
		  AND d.id IN (SELECT DISTINCT device_id FROM expired_subscriptions)
		  AND NOT EXISTS (
			  SELECT 1
			  FROM subscriptions AS active_sub
			  WHERE active_sub.device_id = d.id
			    AND active_sub.status = $6
		  )
	`,
		orgID,
		userID,
		subscriptionID,
		deviceID,
		domain.SubscriptionStatusExpired,
		domain.SubscriptionStatusActive,
		domain.DeviceStatusExpired,
		domain.DeviceStatusSuspended,
	)

	return err
}
