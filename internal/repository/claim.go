package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// ClaimRepository implements domain.ClaimRepository.
type ClaimRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewClaimRepository creates a new claim repository.
func NewClaimRepository(pool *pgxpool.Pool) *ClaimRepository {
	return &ClaimRepository{pool: pool, timeout: 5 * time.Second}
}

const claimColumns = `id, org_id, user_id, device_id, subscription_id, claim_type, description,
       status, amount_minor, market, filed_at, reviewed_at, settled_at, created_at, updated_at`

// Create inserts a new claim.
func (r *ClaimRepository) Create(ctx context.Context, c *domain.Claim) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	market := c.Market
	if market == "" {
		market = domain.MarketSN
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO claims (org_id, user_id, device_id, subscription_id, claim_type, description, status, market)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, filed_at, created_at, updated_at
	`, c.OrgID, c.UserID, c.DeviceID, c.SubscriptionID, c.ClaimType, c.Description, c.Status, market).Scan(&c.ID, &c.FiledAt, &c.CreatedAt, &c.UpdatedAt)
}

// GetByID returns a claim by ID.
func (r *ClaimRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Claim, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var c domain.Claim
	err := r.pool.QueryRow(ctx, `SELECT `+claimColumns+` FROM claims WHERE id = $1`, id).Scan(
		&c.ID, &c.OrgID, &c.UserID, &c.DeviceID, &c.SubscriptionID, &c.ClaimType, &c.Description,
		&c.Status, &c.AmountMinor, &c.Market, &c.FiledAt, &c.ReviewedAt, &c.SettledAt, &c.CreatedAt, &c.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

// ListByOrgAndUser returns claims for a specific org and user.
func (r *ClaimRepository) ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]domain.Claim, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT `+claimColumns+`
		FROM claims
		WHERE org_id = $1 AND user_id = $2
		ORDER BY filed_at DESC
		LIMIT $3 OFFSET $4
	`, orgID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanClaims(rows)
}

// ListByOrg returns all claims in an org, optionally filtered by status.
func (r *ClaimRepository) ListByOrg(ctx context.Context, orgID uuid.UUID, status *string, market string, limit, offset int) ([]domain.Claim, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	statusFilter := ""
	if status != nil {
		statusFilter = *status
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+claimColumns+`
		FROM claims
		WHERE org_id = $1
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR market = $3)
		ORDER BY filed_at DESC
		LIMIT $4 OFFSET $5
	`, orgID, statusFilter, market, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanClaims(rows)
}

// Update modifies a claim.
func (r *ClaimRepository) Update(ctx context.Context, c *domain.Claim) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE claims SET status = $2, amount_minor = $3, reviewed_at = $4, settled_at = $5, updated_at = now()
		WHERE id = $1
	`, c.ID, c.Status, c.AmountMinor, c.ReviewedAt, c.SettledAt)
	return err
}

// ExistsPendingByDeviceAndType checks if a pending/review claim exists for a device and type.
func (r *ClaimRepository) ExistsPendingByDeviceAndType(ctx context.Context, orgID, deviceID uuid.UUID, claimType domain.ClaimType) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM claims
			WHERE org_id = $1 AND device_id = $2 AND claim_type = $3
			AND status IN ('pending', 'review')
		)
	`, orgID, deviceID, claimType).Scan(&exists)
	return exists, err
}

func scanClaims(rows pgx.Rows) ([]domain.Claim, error) {
	var claims []domain.Claim
	for rows.Next() {
		var c domain.Claim
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.UserID, &c.DeviceID, &c.SubscriptionID, &c.ClaimType, &c.Description,
			&c.Status, &c.AmountMinor, &c.Market, &c.FiledAt, &c.ReviewedAt, &c.SettledAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	if claims == nil {
		claims = []domain.Claim{}
	}
	return claims, rows.Err()
}
