package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// PaymentRepository implements domain.PaymentRepository.
type PaymentRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewPaymentRepository creates a new payment repository.
func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool, timeout: 5 * time.Second}
}

const paymentColumns = `id, org_id, user_id, plan_id, subscription_id, amount_minor, market,
       currency, provider, payment_method, status, provider_ref, payment_url,
       idempotency_key, provider_payload, paid_at, failed_at, expires_at, created_at, updated_at`

// Create inserts a new payment.
func (r *PaymentRepository) Create(ctx context.Context, p *domain.Payment) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	market := p.Market
	if market == "" {
		market = domain.MarketSN
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO payments (
			id, org_id, user_id, plan_id, subscription_id, amount_minor, market, currency,
			provider, payment_method, status, provider_ref, payment_url,
			idempotency_key, provider_payload, paid_at, failed_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING created_at, updated_at
	`, p.ID, p.OrgID, p.UserID, p.PlanID, p.SubscriptionID, p.AmountMinor, market, p.Currency,
		p.Provider, p.PaymentMethod, p.Status, p.ProviderRef, p.PaymentURL,
		p.IdempotencyKey, p.ProviderPayload, p.PaidAt, p.FailedAt, p.ExpiresAt,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
}

// GetByID returns a payment by ID.
func (r *PaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.getOne(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id = $1`, id)
}

// GetByIdempotencyKey returns a payment by its idempotency key.
func (r *PaymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.getOne(ctx, `SELECT `+paymentColumns+` FROM payments WHERE idempotency_key = $1`, key)
}

// GetByProviderRef returns a payment by its provider reference.
func (r *PaymentRepository) GetByProviderRef(ctx context.Context, providerRef string) (*domain.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.getOne(ctx, `SELECT `+paymentColumns+` FROM payments WHERE provider_ref = $1`, providerRef)
}

// GetFirstSuccessfulByUser returns the earliest successful payment for a user.
func (r *PaymentRepository) GetFirstSuccessfulByUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.getOne(ctx, `
		SELECT `+paymentColumns+`
		FROM payments
		WHERE org_id = $1
		  AND user_id = $2
		  AND status IN ('completed', 'refunded')
		ORDER BY COALESCE(paid_at, created_at) ASC, created_at ASC, id ASC
		LIMIT 1
	`, orgID, userID)
}

// ListBySubscriptionID returns recent payments for a subscription.
func (r *PaymentRepository) ListBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID, limit int) ([]domain.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+paymentColumns+`
		FROM payments
		WHERE subscription_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, subscriptionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []domain.Payment
	for rows.Next() {
		payment, err := scanPaymentRows(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, *payment)
	}

	if payments == nil {
		payments = []domain.Payment{}
	}
	return payments, rows.Err()
}

// ListByOrgAndUser returns payments for a specific org and user.
func (r *PaymentRepository) ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]domain.Payment, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT `+paymentColumns+`
		FROM payments
		WHERE org_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, orgID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []domain.Payment
	for rows.Next() {
		payment, err := scanPaymentRows(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, *payment)
	}

	if payments == nil {
		payments = []domain.Payment{}
	}
	return payments, rows.Err()
}

// Update modifies a payment.
func (r *PaymentRepository) Update(ctx context.Context, p *domain.Payment) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
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
		WHERE id = $1
	`, p.ID, p.Status, p.PaymentMethod, p.ProviderRef, p.PaymentURL, p.ProviderPayload, p.PaidAt, p.FailedAt, p.ExpiresAt)
	return err
}

func (r *PaymentRepository) getOne(ctx context.Context, query string, args ...any) (*domain.Payment, error) {
	return scanPaymentRow(r.pool.QueryRow(ctx, query, args...))
}

func scanPaymentRow(row pgx.Row) (*domain.Payment, error) {
	var payment domain.Payment
	var payload []byte

	err := row.Scan(
		&payment.ID,
		&payment.OrgID,
		&payment.UserID,
		&payment.PlanID,
		&payment.SubscriptionID,
		&payment.AmountMinor,
		&payment.Market,
		&payment.Currency,
		&payment.Provider,
		&payment.PaymentMethod,
		&payment.Status,
		&payment.ProviderRef,
		&payment.PaymentURL,
		&payment.IdempotencyKey,
		&payload,
		&payment.PaidAt,
		&payment.FailedAt,
		&payment.ExpiresAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	payment.ProviderPayload = payload
	return &payment, nil
}

func scanPaymentRows(rows pgx.Rows) (*domain.Payment, error) {
	var payment domain.Payment
	var payload []byte

	if err := rows.Scan(
		&payment.ID,
		&payment.OrgID,
		&payment.UserID,
		&payment.PlanID,
		&payment.SubscriptionID,
		&payment.AmountMinor,
		&payment.Market,
		&payment.Currency,
		&payment.Provider,
		&payment.PaymentMethod,
		&payment.Status,
		&payment.ProviderRef,
		&payment.PaymentURL,
		&payment.IdempotencyKey,
		&payload,
		&payment.PaidAt,
		&payment.FailedAt,
		&payment.ExpiresAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	); err != nil {
		return nil, err
	}

	payment.ProviderPayload = payload
	return &payment, nil
}
