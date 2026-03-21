package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// AdminRepository implements domain.AdminRepository using pgxpool.
type AdminRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewAdminRepository creates a new admin repository.
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool, timeout: 10 * time.Second}
}

// GetStats returns aggregate platform statistics for the org.
func (r *AdminRepository) GetStats(ctx context.Context, orgID uuid.UUID) (*domain.AdminStats, error) {
	if err := expireEndedSubscriptions(ctx, r.pool, r.timeout, &orgID, nil, nil, nil); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	stats := &domain.AdminStats{
		RevenueByProvider: make(map[string]int),
	}

	var revenueByProvider json.RawMessage
	if err := r.pool.QueryRow(ctx, `
		WITH revenue_by_provider AS (
			SELECT COALESCE(
				jsonb_object_agg(provider, amount_xof),
				'{}'::jsonb
			) AS payload
			FROM (
				SELECT provider, SUM(amount_xof)::int AS amount_xof
				FROM payments
				WHERE org_id = $1
				  AND status = 'completed'
				GROUP BY provider
			) grouped_revenue
		)
		SELECT
			(SELECT COUNT(*)::int FROM subscriptions WHERE org_id = $1 AND status = 'active'),
			(
				SELECT COALESCE(SUM(amount_xof), 0)::int
				FROM payments
				WHERE org_id = $1
				  AND status = 'completed'
				  AND paid_at >= date_trunc('month', now())
			),
			(
				SELECT COUNT(*)::int
				FROM claims
				WHERE org_id = $1
				  AND status NOT IN ('settled', 'rejected')
			),
			(SELECT payload FROM revenue_by_provider),
			(
				SELECT COUNT(*)::int
				FROM users
				WHERE org_id = $1
				  AND deleted_at IS NULL
				  AND role = 'member'
			),
			(
				SELECT COUNT(*)::int
				FROM devices
				WHERE org_id = $1
				  AND deleted_at IS NULL
			)
	`, orgID).Scan(
		&stats.ActiveSubscribers,
		&stats.MonthlyRevenueXOF,
		&stats.OpenClaims,
		&revenueByProvider,
		&stats.TotalCustomers,
		&stats.TotalDevices,
	); err != nil {
		return nil, err
	}

	if len(revenueByProvider) > 0 {
		if err := json.Unmarshal(revenueByProvider, &stats.RevenueByProvider); err != nil {
			return nil, err
		}
	}

	return stats, nil
}

// ListCustomers returns a list of org customers with their subscriptions and device info.
func (r *AdminRepository) ListCustomers(ctx context.Context, orgID uuid.UUID, search string, limit, offset int) ([]domain.AdminCustomer, error) {
	if err := expireEndedSubscriptions(ctx, r.pool, r.timeout, &orgID, nil, nil, nil); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			u.id::text,
			u.full_name,
			u.phone,
			u.email,
			(
				SELECT COUNT(*)
				FROM devices d_count
				WHERE d_count.user_id = u.id
				  AND d_count.org_id = u.org_id
				  AND d_count.deleted_at IS NULL
			) AS device_count,
			(
				SELECT COUNT(*)
				FROM subscriptions s_count
				WHERE s_count.user_id = u.id
				  AND s_count.org_id = u.org_id
				  AND s_count.status = 'active'
			) AS active_subscription_count,
			(
				SELECT COUNT(*)
				FROM subscriptions s_count
				WHERE s_count.user_id = u.id
				  AND s_count.org_id = u.org_id
			) AS total_subscription_count,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'id', s.id::text,
						'plan_id', s.plan_id::text,
						'plan_name_fr', p.name_fr,
						'plan_name_en', p.name_en,
						'status', s.status,
						'billing_cycle', s.billing_cycle,
						'device_id', s.device_id::text,
						'device_brand', d.brand,
						'device_model', d.model,
						'device_type', d.device_type,
						'current_period_start', s.current_period_start,
						'current_period_end', s.current_period_end,
						'created_at', s.created_at,
						'updated_at', s.updated_at
					)
					ORDER BY
						CASE s.status
							WHEN 'active' THEN 0
							WHEN 'pending' THEN 1
							WHEN 'expired' THEN 2
							WHEN 'cancelled' THEN 3
							ELSE 4
						END,
						COALESCE(s.current_period_end, s.created_at) DESC NULLS LAST,
						s.created_at DESC
				) FILTER (WHERE s.id IS NOT NULL),
				'[]'::jsonb
			) AS subscriptions
		FROM users u
		LEFT JOIN subscriptions s ON s.user_id = u.id AND s.org_id = u.org_id
		LEFT JOIN plans p ON p.id = s.plan_id
		LEFT JOIN devices d ON d.id = s.device_id AND d.org_id = s.org_id AND d.deleted_at IS NULL
		WHERE u.org_id = $1
		  AND u.deleted_at IS NULL
		  AND ($2 = '' OR lower(u.full_name) LIKE '%' || lower($2) || '%' OR lower(u.email) LIKE '%' || lower($2) || '%')
		  AND u.role = 'member'
		GROUP BY u.id, u.full_name, u.phone, u.email, u.created_at
		ORDER BY u.created_at DESC
		LIMIT $3 OFFSET $4
	`, orgID, search, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []domain.AdminCustomer
	for rows.Next() {
		var c domain.AdminCustomer
		var subscriptions json.RawMessage

		if err := rows.Scan(
			&c.ID,
			&c.FullName,
			&c.Phone,
			&c.Email,
			&c.DeviceCount,
			&c.ActiveSubscriptionCount,
			&c.TotalSubscriptionCount,
			&subscriptions,
		); err != nil {
			return nil, err
		}

		if len(subscriptions) > 0 {
			if err := json.Unmarshal(subscriptions, &c.Subscriptions); err != nil {
				return nil, err
			}
		}
		if c.Subscriptions == nil {
			c.Subscriptions = []domain.AdminCustomerSubscription{}
		}

		customers = append(customers, c)
	}
	if customers == nil {
		customers = []domain.AdminCustomer{}
	}
	return customers, rows.Err()
}

// ListPayments returns all payments in the org with enriched customer and plan info.
func (r *AdminRepository) ListPayments(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]domain.AdminPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			pay.id::text,
			u.full_name,
			pl.name_fr,
			pl.name_en,
			pay.amount_xof,
			pay.provider,
			pay.payment_method,
			pay.status,
			pay.paid_at,
			pay.created_at
		FROM payments pay
		JOIN users u ON u.id = pay.user_id
		JOIN plans pl ON pl.id = pay.plan_id
		WHERE pay.org_id = $1
		ORDER BY pay.created_at DESC
		LIMIT $2 OFFSET $3
	`, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []domain.AdminPayment
	for rows.Next() {
		var p domain.AdminPayment
		if err := rows.Scan(&p.ID, &p.CustomerName, &p.PlanNameFR, &p.PlanNameEN, &p.AmountXOF, &p.Provider, &p.PaymentMethod, &p.Status, &p.PaidAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	if payments == nil {
		payments = []domain.AdminPayment{}
	}
	return payments, rows.Err()
}
