package repository

import (
	"context"
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
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	stats := &domain.AdminStats{
		RevenueByProvider: make(map[string]int),
	}

	// Active subscribers
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM subscriptions WHERE org_id = $1 AND status = 'active'
	`, orgID).Scan(&stats.ActiveSubscribers); err != nil {
		return nil, err
	}

	// Monthly revenue (current calendar month)
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_xof), 0)
		FROM payments
		WHERE org_id = $1 AND status = 'completed'
		  AND paid_at >= date_trunc('month', now())
	`, orgID).Scan(&stats.MonthlyRevenueXOF); err != nil {
		return nil, err
	}

	// Open claims
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM claims
		WHERE org_id = $1 AND status NOT IN ('settled', 'rejected')
	`, orgID).Scan(&stats.OpenClaims); err != nil {
		return nil, err
	}

	// Revenue by provider (all-time). DEXPAY sub-methods are only shown separately
	// when they are explicitly confirmed by provider payloads.
	rows, err := r.pool.Query(ctx, `
		SELECT provider, COALESCE(SUM(amount_xof), 0)
		FROM payments
		WHERE org_id = $1 AND status = 'completed'
		GROUP BY provider
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var provider string
		var amount int
		if err := rows.Scan(&provider, &amount); err != nil {
			return nil, err
		}
		stats.RevenueByProvider[provider] = amount
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Total customers (non-admin, non-deleted)
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE org_id = $1 AND deleted_at IS NULL AND role = 'member'
	`, orgID).Scan(&stats.TotalCustomers); err != nil {
		return nil, err
	}

	// Total registered devices
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM devices WHERE org_id = $1 AND deleted_at IS NULL
	`, orgID).Scan(&stats.TotalDevices); err != nil {
		return nil, err
	}

	return stats, nil
}

// ListCustomers returns a list of org customers with their plan and device info.
func (r *AdminRepository) ListCustomers(ctx context.Context, orgID uuid.UUID, search string, limit, offset int) ([]domain.AdminCustomer, error) {
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
			p.name_fr,
			p.name_en,
			COUNT(DISTINCT d.id) AS device_count,
			s.status
		FROM users u
		LEFT JOIN subscriptions s ON s.user_id = u.id AND s.org_id = u.org_id
			AND s.status = 'active'
		LEFT JOIN plans p ON p.id = s.plan_id
		LEFT JOIN devices d ON d.user_id = u.id AND d.org_id = u.org_id AND d.deleted_at IS NULL
		WHERE u.org_id = $1
		  AND u.deleted_at IS NULL
		  AND ($2 = '' OR lower(u.full_name) LIKE '%' || lower($2) || '%' OR lower(u.email) LIKE '%' || lower($2) || '%')
		GROUP BY u.id, u.full_name, u.phone, u.email, p.name_fr, p.name_en, s.status
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
		if err := rows.Scan(&c.ID, &c.FullName, &c.Phone, &c.Email, &c.PlanNameFR, &c.PlanNameEN, &c.DeviceCount, &c.SubscriptionStatus); err != nil {
			return nil, err
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
