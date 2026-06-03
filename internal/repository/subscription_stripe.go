package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// CreateStripeSubscriptionParams creates a US subscription from a verified
// Stripe webhook. device_id stays NULL until the user finishes
// /us/register-device after checkout.
type CreateStripeSubscriptionParams struct {
	OrgID                   uuid.UUID
	UserID                  uuid.UUID
	PlanID                  uuid.UUID
	BillingCycle            string
	Status                  domain.SubscriptionStatus
	StripeSubscriptionID    string
	StripeCheckoutSessionID string
	CurrentPeriodStart      *time.Time
	CurrentPeriodEnd        *time.Time
}

// CreateStripeSubscription inserts a US subscription. Idempotent on
// stripe_subscription_id thanks to the partial unique index — re-processed
// webhook events update the existing row instead of duplicating it.
func (r *SubscriptionRepository) CreateStripeSubscription(
	ctx context.Context,
	p CreateStripeSubscriptionParams,
) (*domain.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var s domain.Subscription
	err := r.pool.QueryRow(ctx, `
		INSERT INTO subscriptions (
			org_id, user_id, plan_id, status, billing_cycle, market,
			stripe_subscription_id, stripe_checkout_session_id,
			current_period_start, current_period_end
		)
		VALUES ($1, $2, $3, $4, $5, 'US', $6, $7, $8, $9)
		ON CONFLICT (stripe_subscription_id) WHERE stripe_subscription_id IS NOT NULL
		DO UPDATE SET
			status = EXCLUDED.status,
			current_period_start = EXCLUDED.current_period_start,
			current_period_end = EXCLUDED.current_period_end,
			updated_at = now()
		RETURNING id, org_id, user_id, plan_id, status, billing_cycle,
			current_period_start, current_period_end, cancelled_at,
			created_at, updated_at
	`,
		p.OrgID, p.UserID, p.PlanID, p.Status, p.BillingCycle,
		p.StripeSubscriptionID, p.StripeCheckoutSessionID,
		p.CurrentPeriodStart, p.CurrentPeriodEnd,
	).Scan(
		&s.ID, &s.OrgID, &s.UserID, &s.PlanID, &s.Status, &s.BillingCycle,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelledAt,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateStripeSubscriptionState applies a Stripe webhook event to an
// existing subscription. Returns pgx.ErrNoRows if no row matches.
func (r *SubscriptionRepository) UpdateStripeSubscriptionState(
	ctx context.Context,
	stripeSubscriptionID string,
	status domain.SubscriptionStatus,
	periodStart, periodEnd, cancelledAt *time.Time,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	tag, err := r.pool.Exec(ctx, `
		UPDATE subscriptions SET
			status = $2,
			current_period_start = COALESCE($3, current_period_start),
			current_period_end = COALESCE($4, current_period_end),
			cancelled_at = COALESCE($5, cancelled_at),
			updated_at = now()
		WHERE stripe_subscription_id = $1
	`, stripeSubscriptionID, status, periodStart, periodEnd, cancelledAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// FindUSPendingSubscriptionWithoutDevice returns the user's most-recent US
// subscription that has no device attached. Used post-checkout to attach
// the device the user fills in /us/register-device.
func (r *SubscriptionRepository) FindUSPendingSubscriptionWithoutDevice(
	ctx context.Context,
	userID uuid.UUID,
) (*domain.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var s domain.Subscription
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, plan_id, status, billing_cycle,
		       current_period_start, current_period_end, cancelled_at,
		       created_at, updated_at
		FROM subscriptions
		WHERE user_id = $1
		  AND market = 'US'
		  AND device_id IS NULL
		  AND status IN ('pending', 'active', 'past_due')
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&s.ID, &s.OrgID, &s.UserID, &s.PlanID, &s.Status, &s.BillingCycle,
		&s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelledAt,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetByStripeSubscriptionID returns the subscription row keyed by Stripe's
// subscription ID. Used by invoice webhooks to find the owning user / org /
// plan when recording a Payment row.
func (r *SubscriptionRepository) GetByStripeSubscriptionID(
	ctx context.Context,
	stripeSubscriptionID string,
) (*domain.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var s domain.Subscription
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, COALESCE(device_id, '00000000-0000-0000-0000-000000000000'::uuid), plan_id,
		       status, billing_cycle, market, activated_at, current_period_start, current_period_end,
		       cancelled_at, created_at, updated_at
		FROM subscriptions
		WHERE stripe_subscription_id = $1
	`, stripeSubscriptionID).Scan(
		&s.ID, &s.OrgID, &s.UserID, &s.DeviceID, &s.PlanID,
		&s.Status, &s.BillingCycle, &s.Market, &s.ActivatedAt, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.CancelledAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// AttachDevice links a device to a subscription. Idempotent: no-op if the
// subscription already has a device.
func (r *SubscriptionRepository) AttachDevice(
	ctx context.Context,
	subscriptionID, deviceID uuid.UUID,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	tag, err := r.pool.Exec(ctx, `
		UPDATE subscriptions
		SET device_id = $2, updated_at = now()
		WHERE id = $1 AND device_id IS NULL
	`, subscriptionID, deviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
