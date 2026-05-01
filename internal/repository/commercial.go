package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// CommercialRepository implements commercial acquisition persistence.
type CommercialRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewCommercialRepository creates a commercial repository.
func NewCommercialRepository(pool *pgxpool.Pool) *CommercialRepository {
	return &CommercialRepository{pool: pool, timeout: 5 * time.Second}
}

// CreateProfile inserts a commercial profile.
func (r *CommercialRepository) CreateProfile(ctx context.Context, profile *domain.CommercialProfile) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO commercial_profiles (
			org_id, user_id, referral_code, status, commission_percentage
		)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'active'), $5)
		ON CONFLICT (org_id, user_id) DO UPDATE
		SET status = EXCLUDED.status,
		    commission_percentage = EXCLUDED.commission_percentage,
		    updated_at = now()
		RETURNING id, org_id, user_id, referral_code, status, commission_percentage, created_at, updated_at
	`, profile.OrgID, profile.UserID, profile.ReferralCode, profile.Status, profile.CommissionPercentage).Scan(
		&profile.ID,
		&profile.OrgID,
		&profile.UserID,
		&profile.ReferralCode,
		&profile.Status,
		&profile.CommissionPercentage,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
}

func (r *CommercialRepository) GetProfileByUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.CommercialProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return scanCommercialProfile(r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, referral_code, status, commission_percentage, created_at, updated_at
		FROM commercial_profiles
		WHERE org_id = $1 AND user_id = $2
	`, orgID, userID))
}

func (r *CommercialRepository) GetProfileByID(ctx context.Context, orgID, commercialID uuid.UUID) (*domain.CommercialProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return scanCommercialProfile(r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, referral_code, status, commission_percentage, created_at, updated_at
		FROM commercial_profiles
		WHERE org_id = $1 AND id = $2
	`, orgID, commercialID))
}

func (r *CommercialRepository) GetProfileByReferralCode(ctx context.Context, orgID uuid.UUID, code string) (*domain.CommercialProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return scanCommercialProfile(r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, referral_code, status, commission_percentage, created_at, updated_at
		FROM commercial_profiles
		WHERE org_id = $1 AND referral_code = $2
	`, orgID, code))
}

func (r *CommercialRepository) ListPartners(ctx context.Context, orgID, commercialID uuid.UUID, limit, offset int) ([]domain.CommercialPartner, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			p.id::text,
			p.store_name,
			u.full_name,
			u.email,
			u.phone,
			p.city,
			p.business_location,
			p.status,
			pa.status,
			pa.reviewed_at,
			p.commission_percentage,
			COALESCE(client_stats.clients_count, 0),
			COALESCE(client_stats.active_clients, 0),
			first_payment.status,
			p.created_at
		FROM partners p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN partner_applications pa
		  ON pa.org_id = p.org_id
		 AND pa.user_id = p.user_id
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*)::int AS clients_count,
				COUNT(*) FILTER (WHERE status = 'active')::int AS active_clients
			FROM partner_clients pc
			WHERE pc.partner_id = p.id
		) AS client_stats ON TRUE
		LEFT JOIN LATERAL (
			SELECT pay.status::text AS status
			FROM partner_clients pc
			JOIN payments pay
			  ON pay.org_id = pc.org_id
			 AND pay.user_id = pc.linked_user_id
			WHERE pc.partner_id = p.id
			  AND pay.status IN ('completed', 'refunded')
			ORDER BY COALESCE(pay.paid_at, pay.created_at) ASC, pay.created_at ASC, pay.id ASC
			LIMIT 1
		) AS first_payment ON TRUE
		WHERE p.org_id = $1 AND p.commercial_id = $2
		ORDER BY p.created_at DESC
		LIMIT $3 OFFSET $4
	`, orgID, commercialID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []domain.CommercialPartner
	for rows.Next() {
		var item domain.CommercialPartner
		if err := rows.Scan(
			&item.ID,
			&item.StoreName,
			&item.OwnerName,
			&item.OwnerEmail,
			&item.Phone,
			&item.City,
			&item.BusinessLocation,
			&item.Status,
			&item.ApplicationStatus,
			&item.ApprovalDate,
			&item.PartnerCommissionRate,
			&item.ClientsCount,
			&item.ActiveClients,
			&item.FirstPaymentStatus,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		partners = append(partners, item)
	}
	if partners == nil {
		partners = []domain.CommercialPartner{}
	}
	return partners, rows.Err()
}

func (r *CommercialRepository) ListCommissions(ctx context.Context, orgID, commercialID uuid.UUID, limit, offset int) ([]domain.CommercialCommissionView, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, commercialCommissionSelect(`
		WHERE cc.org_id = $1 AND cc.commercial_id = $2
		ORDER BY cc.created_at DESC
		LIMIT $3 OFFSET $4
	`), orgID, commercialID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCommercialCommissionViews(rows)
}

func (r *CommercialRepository) ListActivityReports(ctx context.Context, orgID uuid.UUID, commercialID *uuid.UUID, partnerID *uuid.UUID, limit, offset int) ([]domain.CommercialActivityReportView, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT
			car.id::text,
			car.commercial_id::text,
			cu.full_name,
			car.partner_id::text,
			p.store_name,
			p.status,
			car.prospect_name,
			car.activity_type,
			car.photo_url,
			car.comment,
			car.city,
			car.location,
			car.created_at
		FROM commercial_activity_reports car
		JOIN commercial_profiles cp ON cp.id = car.commercial_id
		JOIN users cu ON cu.id = cp.user_id
		LEFT JOIN partners p ON p.id = car.partner_id
		WHERE car.org_id = $1
	`
	args := []any{orgID, limit, offset}
	nextArg := 4
	if commercialID != nil {
		query += " AND car.commercial_id = $" + itoa(nextArg)
		args = append(args, *commercialID)
		nextArg++
	}
	if partnerID != nil {
		query += " AND car.partner_id = $" + itoa(nextArg)
		args = append(args, *partnerID)
	}
	query += " ORDER BY car.created_at DESC LIMIT $2 OFFSET $3"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []domain.CommercialActivityReportView
	for rows.Next() {
		var item domain.CommercialActivityReportView
		if err := rows.Scan(
			&item.ID,
			&item.CommercialID,
			&item.CommercialName,
			&item.PartnerID,
			&item.PartnerStoreName,
			&item.PartnerStatus,
			&item.ProspectName,
			&item.ActivityType,
			&item.PhotoURL,
			&item.Comment,
			&item.City,
			&item.Location,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		reports = append(reports, item)
	}
	if reports == nil {
		reports = []domain.CommercialActivityReportView{}
	}
	return reports, rows.Err()
}

func (r *CommercialRepository) CreateActivityReport(ctx context.Context, report *domain.CommercialActivityReport) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO commercial_activity_reports (
			id, org_id, commercial_id, partner_id, prospect_name, activity_type,
			photo_url, photo_storage_path, photo_content_type, comment, city, location
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at
	`, report.ID, report.OrgID, report.CommercialID, report.PartnerID, report.ProspectName, report.ActivityType,
		report.PhotoURL, report.PhotoStoragePath, report.PhotoContentType, report.Comment, report.City, report.Location,
	).Scan(&report.ID, &report.CreatedAt)
}

func (r *CommercialRepository) GetActivityReport(ctx context.Context, orgID, reportID uuid.UUID) (*domain.CommercialActivityReport, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var item domain.CommercialActivityReport
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, commercial_id, partner_id, prospect_name, activity_type,
		       photo_url, photo_storage_path, photo_content_type, comment, city, location, created_at
		FROM commercial_activity_reports
		WHERE org_id = $1 AND id = $2
	`, orgID, reportID).Scan(
		&item.ID,
		&item.OrgID,
		&item.CommercialID,
		&item.PartnerID,
		&item.ProspectName,
		&item.ActivityType,
		&item.PhotoURL,
		&item.PhotoStoragePath,
		&item.PhotoContentType,
		&item.Comment,
		&item.City,
		&item.Location,
		&item.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CommercialRepository) ListAdminCommercials(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]domain.AdminCommercialListItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, adminCommercialSelect(`
		WHERE cp.org_id = $1
		ORDER BY cp.created_at DESC
		LIMIT $2 OFFSET $3
	`), orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.AdminCommercialListItem
	for rows.Next() {
		item, err := scanAdminCommercialListItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if items == nil {
		items = []domain.AdminCommercialListItem{}
	}
	return items, rows.Err()
}

func (r *CommercialRepository) GetAdminCommercial(ctx context.Context, orgID, commercialID uuid.UUID) (*domain.AdminCommercialDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	row := r.pool.QueryRow(ctx, adminCommercialSelect(`
		WHERE cp.org_id = $1 AND cp.id = $2
	`), orgID, commercialID)
	commercial, err := scanAdminCommercialListItem(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	partners, err := r.ListPartners(ctx, orgID, commercialID, 100, 0)
	if err != nil {
		return nil, err
	}
	reports, err := r.ListActivityReports(ctx, orgID, &commercialID, nil, 100, 0)
	if err != nil {
		return nil, err
	}
	commissions, err := r.ListCommissions(ctx, orgID, commercialID, 100, 0)
	if err != nil {
		return nil, err
	}

	return &domain.AdminCommercialDetail{
		Commercial:  *commercial,
		Partners:    partners,
		Reports:     reports,
		Commissions: commissions,
	}, nil
}

func (r *CommercialRepository) UpdateStatus(ctx context.Context, orgID, commercialID uuid.UUID, status string) (*domain.CommercialProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return scanCommercialProfile(r.pool.QueryRow(ctx, `
		UPDATE commercial_profiles
		SET status = $3, updated_at = now()
		WHERE org_id = $1 AND id = $2
		RETURNING id, org_id, user_id, referral_code, status, commission_percentage, created_at, updated_at
	`, orgID, commercialID, status))
}

func (r *CommercialRepository) UpdateCommissionPercentage(ctx context.Context, orgID, commercialID uuid.UUID, percentage float64) (*domain.CommercialProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return scanCommercialProfile(r.pool.QueryRow(ctx, `
		UPDATE commercial_profiles
		SET commission_percentage = $3, updated_at = now()
		WHERE org_id = $1 AND id = $2
		RETURNING id, org_id, user_id, referral_code, status, commission_percentage, created_at, updated_at
	`, orgID, commercialID, percentage))
}

func (r *CommercialRepository) CreateCommissionForFirstPartnerPayment(ctx context.Context, commission *domain.CommercialCommission) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	err := r.pool.QueryRow(ctx, `
		INSERT INTO commercial_commissions (
			org_id, commercial_id, partner_id, partner_client_id, client_user_id, payment_id, plan_id,
			base_amount_xof, commission_percentage, commission_amount_xof, status
		)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		WHERE $6 = (
			SELECT pay.id
			FROM partner_clients pc
			JOIN payments pay
			  ON pay.org_id = pc.org_id
			 AND pay.user_id = pc.linked_user_id
			WHERE pc.partner_id = $3
			  AND pay.status IN ('completed', 'refunded')
			ORDER BY COALESCE(pay.paid_at, pay.created_at) ASC, pay.created_at ASC, pay.id ASC
			LIMIT 1
		)
		ON CONFLICT DO NOTHING
		RETURNING id, created_at, updated_at
	`, commission.OrgID, commission.CommercialID, commission.PartnerID, commission.PartnerClientID,
		commission.ClientUserID, commission.PaymentID, commission.PlanID, commission.BaseAmountXOF,
		commission.CommissionPercentage, commission.CommissionAmountXOF, commission.Status,
	).Scan(&commission.ID, &commission.CreatedAt, &commission.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil
	}
	return err
}

func scanCommercialProfile(row pgx.Row) (*domain.CommercialProfile, error) {
	var profile domain.CommercialProfile
	err := row.Scan(
		&profile.ID,
		&profile.OrgID,
		&profile.UserID,
		&profile.ReferralCode,
		&profile.Status,
		&profile.CommissionPercentage,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func adminCommercialSelect(suffix string) string {
	return `
		SELECT
			cp.id::text,
			cp.user_id::text,
			u.full_name,
			u.email,
			u.phone,
			cp.status,
			cp.referral_code,
			cp.commission_percentage,
			COALESCE(partner_stats.partners_brought, 0),
			COALESCE(partner_stats.approved_partners, 0),
			COALESCE(application_stats.pending_partners, 0),
			COALESCE(commission_stats.commission_earned_xof, 0),
			activity_stats.last_activity_date,
			cp.created_at
		FROM commercial_profiles cp
		JOIN users u ON u.id = cp.user_id
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*)::int AS partners_brought,
				COUNT(*) FILTER (WHERE status IN ('active', 'approved'))::int AS approved_partners
			FROM partners p
			WHERE p.commercial_id = cp.id
		) AS partner_stats ON TRUE
		LEFT JOIN LATERAL (
			SELECT COUNT(*)::int AS pending_partners
			FROM partner_applications pa
			WHERE pa.commercial_id = cp.id AND pa.status = 'pending'
		) AS application_stats ON TRUE
		LEFT JOIN LATERAL (
			SELECT SUM(commission_amount_xof)::int AS commission_earned_xof
			FROM commercial_commissions cc
			WHERE cc.commercial_id = cp.id
		) AS commission_stats ON TRUE
		LEFT JOIN LATERAL (
			SELECT MAX(created_at) AS last_activity_date
			FROM commercial_activity_reports car
			WHERE car.commercial_id = cp.id
		) AS activity_stats ON TRUE
	` + suffix
}

func scanAdminCommercialListItem(row pgx.Row) (*domain.AdminCommercialListItem, error) {
	var item domain.AdminCommercialListItem
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Name,
		&item.Email,
		&item.Phone,
		&item.Status,
		&item.ReferralCode,
		&item.CommissionPercentage,
		&item.PartnersBrought,
		&item.ApprovedPartners,
		&item.PendingPartners,
		&item.CommissionEarnedXOF,
		&item.LastActivityDate,
		&item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func commercialCommissionSelect(suffix string) string {
	return `
		SELECT
			cc.id::text,
			cc.commercial_id::text,
			cu.full_name AS commercial_name,
			cc.partner_id::text,
			p.store_name,
			cc.partner_client_id::text,
			cc.client_user_id::text,
			COALESCE(pc.client_name, client.full_name, '-') AS client_name,
			cc.payment_id::text,
			cc.plan_id::text,
			pl.name_fr,
			pl.name_en,
			cc.base_amount_xof,
			cc.commission_percentage,
			cc.commission_amount_xof,
			cc.status,
			cc.paid_at,
			cc.created_at
		FROM commercial_commissions cc
		JOIN commercial_profiles cp ON cp.id = cc.commercial_id
		JOIN users cu ON cu.id = cp.user_id
		JOIN partners p ON p.id = cc.partner_id
		LEFT JOIN partner_clients pc ON pc.id = cc.partner_client_id
		LEFT JOIN users client ON client.id = cc.client_user_id
		LEFT JOIN plans pl ON pl.id = cc.plan_id
	` + suffix
}

func scanCommercialCommissionViews(rows pgx.Rows) ([]domain.CommercialCommissionView, error) {
	var items []domain.CommercialCommissionView
	for rows.Next() {
		var item domain.CommercialCommissionView
		if err := rows.Scan(
			&item.ID,
			&item.CommercialID,
			&item.CommercialName,
			&item.PartnerID,
			&item.PartnerStoreName,
			&item.PartnerClientID,
			&item.ClientUserID,
			&item.ClientName,
			&item.PaymentID,
			&item.PlanID,
			&item.PlanNameFR,
			&item.PlanNameEN,
			&item.BaseAmountXOF,
			&item.CommissionPercentage,
			&item.CommissionAmountXOF,
			&item.Status,
			&item.PaidAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = []domain.CommercialCommissionView{}
	}
	return items, rows.Err()
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
