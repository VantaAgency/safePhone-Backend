package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// PartnerApplicationRepository implements domain.PartnerApplicationRepository using pgxpool.
type PartnerApplicationRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewPartnerApplicationRepository creates a new partner application repository.
func NewPartnerApplicationRepository(pool *pgxpool.Pool) *PartnerApplicationRepository {
	return &PartnerApplicationRepository{pool: pool, timeout: 5 * time.Second}
}

// Create inserts a new partner application.
func (r *PartnerApplicationRepository) Create(ctx context.Context, app *domain.PartnerApplication) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO partner_applications (
			org_id, user_id, store_name, full_name, phone, city, business_location,
			commercial_id, acquisition_source
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, status, created_at
	`, app.OrgID, app.UserID, app.StoreName, app.FullName, app.Phone, app.City, app.BusinessLocation, app.CommercialID, app.AcquisitionSource).Scan(&app.ID, &app.Status, &app.CreatedAt)
}

// GetByID returns a partner application by its ID.
func (r *PartnerApplicationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PartnerApplication, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	app := &domain.PartnerApplication{}
	err := r.pool.QueryRow(ctx, `
		SELECT pa.id, pa.org_id, pa.user_id, pa.store_name, pa.full_name, pa.phone, pa.city,
		       pa.business_location, pa.status, p.commission_percentage,
		       pa.commercial_id, cu.full_name AS commercial_name, pa.acquisition_source,
		       pa.reviewed_by, pa.rejection_reason, pa.created_at, pa.reviewed_at
		FROM partner_applications pa
		LEFT JOIN partners p
		  ON p.org_id = pa.org_id
		 AND p.user_id = pa.user_id
		LEFT JOIN commercial_profiles cp ON cp.id = pa.commercial_id
		LEFT JOIN users cu ON cu.id = cp.user_id
		WHERE pa.id = $1
	`, id).Scan(
		&app.ID, &app.OrgID, &app.UserID, &app.StoreName, &app.FullName, &app.Phone, &app.City,
		&app.BusinessLocation, &app.Status, &app.CommissionPercentage, &app.CommercialID, &app.CommercialName, &app.AcquisitionSource, &app.ReviewedBy,
		&app.RejectionReason, &app.CreatedAt, &app.ReviewedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return app, nil
}

// GetByUser returns the latest pending application for a user.
func (r *PartnerApplicationRepository) GetByUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.PartnerApplication, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	app := &domain.PartnerApplication{}
	err := r.pool.QueryRow(ctx, `
		SELECT pa.id, pa.org_id, pa.user_id, pa.store_name, pa.full_name, pa.phone, pa.city,
		       pa.business_location, pa.status, p.commission_percentage,
		       pa.commercial_id, cu.full_name AS commercial_name, pa.acquisition_source,
		       pa.reviewed_by, pa.rejection_reason, pa.created_at, pa.reviewed_at
		FROM partner_applications pa
		LEFT JOIN partners p
		  ON p.org_id = pa.org_id
		 AND p.user_id = pa.user_id
		LEFT JOIN commercial_profiles cp ON cp.id = pa.commercial_id
		LEFT JOIN users cu ON cu.id = cp.user_id
		WHERE pa.org_id = $1 AND pa.user_id = $2
		ORDER BY pa.created_at DESC
		LIMIT 1
	`, orgID, userID).Scan(
		&app.ID, &app.OrgID, &app.UserID, &app.StoreName, &app.FullName, &app.Phone, &app.City,
		&app.BusinessLocation, &app.Status, &app.CommissionPercentage, &app.CommercialID, &app.CommercialName, &app.AcquisitionSource, &app.ReviewedBy,
		&app.RejectionReason, &app.CreatedAt, &app.ReviewedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return app, nil
}

// ListByOrg returns partner applications for an org, optionally filtered by status.
func (r *PartnerApplicationRepository) ListByOrg(ctx context.Context, orgID uuid.UUID, status *string, limit, offset int) ([]domain.AdminPartnerApplication, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT pa.id::text, pa.org_id::text, pa.user_id::text,
		       pa.store_name, pa.full_name, pa.phone, u.email, pa.city, pa.business_location,
		       pa.status, p.commission_percentage, pa.rejection_reason, pa.created_at, pa.reviewed_at
		FROM partner_applications pa
		JOIN users u ON u.id = pa.user_id
		LEFT JOIN partners p
		  ON p.org_id = pa.org_id
		 AND p.user_id = pa.user_id
		WHERE pa.org_id = $1
	`
	args := []any{orgID}

	if status != nil {
		query += " AND pa.status = $4"
		args = append(args, *status)
	}

	query += " ORDER BY pa.created_at DESC LIMIT $2 OFFSET $3"
	// Insert limit/offset at positions $2 and $3
	// Need to rewrite args ordering
	if status != nil {
		query = `
			SELECT pa.id::text, pa.org_id::text, pa.user_id::text,
			       pa.store_name, pa.full_name, pa.phone, u.email, pa.city, pa.business_location,
			       pa.status, p.commission_percentage, pa.commercial_id::text, cu.full_name AS commercial_name,
			       pa.acquisition_source, pa.rejection_reason, pa.created_at, pa.reviewed_at
			FROM partner_applications pa
			JOIN users u ON u.id = pa.user_id
			LEFT JOIN partners p
			  ON p.org_id = pa.org_id
			 AND p.user_id = pa.user_id
			LEFT JOIN commercial_profiles cp ON cp.id = pa.commercial_id
			LEFT JOIN users cu ON cu.id = cp.user_id
			WHERE pa.org_id = $1 AND pa.status = $4
			ORDER BY pa.created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []any{orgID, limit, offset, *status}
	} else {
		query = `
			SELECT pa.id::text, pa.org_id::text, pa.user_id::text,
			       pa.store_name, pa.full_name, pa.phone, u.email, pa.city, pa.business_location,
			       pa.status, p.commission_percentage, pa.commercial_id::text, cu.full_name AS commercial_name,
			       pa.acquisition_source, pa.rejection_reason, pa.created_at, pa.reviewed_at
			FROM partner_applications pa
			JOIN users u ON u.id = pa.user_id
			LEFT JOIN partners p
			  ON p.org_id = pa.org_id
			 AND p.user_id = pa.user_id
			LEFT JOIN commercial_profiles cp ON cp.id = pa.commercial_id
			LEFT JOIN users cu ON cu.id = cp.user_id
			WHERE pa.org_id = $1
			ORDER BY pa.created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []any{orgID, limit, offset}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []domain.AdminPartnerApplication
	for rows.Next() {
		var a domain.AdminPartnerApplication
		if err := rows.Scan(
			&a.ID, &a.OrgID, &a.UserID,
			&a.StoreName, &a.FullName, &a.Phone, &a.Email, &a.City, &a.BusinessLocation,
			&a.Status, &a.CommissionPercentage, &a.CommercialID, &a.CommercialName, &a.AcquisitionSource, &a.RejectionReason, &a.CreatedAt, &a.ReviewedAt,
		); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	if apps == nil {
		apps = []domain.AdminPartnerApplication{}
	}
	return apps, rows.Err()
}

// UpdateStatus updates the status, reviewed_by, rejection_reason, and reviewed_at of an application.
func (r *PartnerApplicationRepository) UpdateStatus(ctx context.Context, app *domain.PartnerApplication) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE partner_applications
		SET status = $2, reviewed_by = $3, rejection_reason = $4, reviewed_at = $5
		WHERE id = $1
	`, app.ID, app.Status, app.ReviewedBy, app.RejectionReason, app.ReviewedAt)
	return err
}
