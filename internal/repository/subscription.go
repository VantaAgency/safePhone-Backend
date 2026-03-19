package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// SubscriptionRepository implements domain.SubscriptionRepository.
type SubscriptionRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewSubscriptionRepository creates a new subscription repository.
func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{pool: pool, timeout: 5 * time.Second}
}

const subColumns = `id, org_id, user_id, device_id, plan_id, status, billing_cycle,
       current_period_start, current_period_end, cancelled_at,
       created_at, updated_at`

// Create inserts a new subscription.
func (r *SubscriptionRepository) Create(ctx context.Context, s *domain.Subscription) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO subscriptions (org_id, user_id, device_id, plan_id, status, billing_cycle,
		       current_period_start, current_period_end)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`, s.OrgID, s.UserID, s.DeviceID, s.PlanID, s.Status, s.BillingCycle,
		s.CurrentPeriodStart, s.CurrentPeriodEnd).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

// GetByID returns a subscription by ID.
func (r *SubscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var s domain.Subscription
	err := r.pool.QueryRow(ctx, `SELECT `+subColumns+` FROM subscriptions WHERE id = $1`, id).Scan(
		&s.ID, &s.OrgID, &s.UserID, &s.DeviceID, &s.PlanID, &s.Status, &s.BillingCycle,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelledAt,
		&s.CreatedAt, &s.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

// GetByDeviceID returns the most recent subscription for a device.
func (r *SubscriptionRepository) GetByDeviceID(ctx context.Context, deviceID uuid.UUID) (*domain.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var s domain.Subscription
	err := r.pool.QueryRow(ctx, `
		SELECT `+subColumns+`
		FROM subscriptions
		WHERE device_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, deviceID).Scan(
		&s.ID, &s.OrgID, &s.UserID, &s.DeviceID, &s.PlanID, &s.Status, &s.BillingCycle,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelledAt,
		&s.CreatedAt, &s.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

// ListByOrgAndUser returns subscriptions for a specific org and user.
func (r *SubscriptionRepository) ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]domain.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT `+subColumns+`
		FROM subscriptions
		WHERE org_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, orgID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		if err := rows.Scan(
			&s.ID, &s.OrgID, &s.UserID, &s.DeviceID, &s.PlanID, &s.Status, &s.BillingCycle,
			&s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelledAt,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}

	if subs == nil {
		subs = []domain.Subscription{}
	}
	return subs, rows.Err()
}

// Update modifies a subscription.
func (r *SubscriptionRepository) Update(ctx context.Context, s *domain.Subscription) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE subscriptions
		SET status = $2, cancelled_at = $3,
		    current_period_start = $4, current_period_end = $5,
		    updated_at = now()
		WHERE id = $1
	`, s.ID, s.Status, s.CancelledAt,
		s.CurrentPeriodStart, s.CurrentPeriodEnd)
	return err
}
