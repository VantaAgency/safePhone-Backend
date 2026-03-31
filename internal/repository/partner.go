package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

const partnerClientColumns = `
	id, org_id, partner_id, linked_user_id, client_name, client_phone, plan_id, status,
	attribution_source, referral_code, referral_medium, attributed_at,
	invitation_token, invitation_expires_at, invitation_claimed_at, invited_at, created_at, updated_at
`

const partnerClientColumnsQualified = `
	pc.id, pc.org_id, pc.partner_id, pc.linked_user_id, pc.client_name, pc.client_phone, pc.plan_id, pc.status,
	pc.attribution_source, pc.referral_code, pc.referral_medium, pc.attributed_at,
	pc.invitation_token, pc.invitation_expires_at, pc.invitation_claimed_at, pc.invited_at, pc.created_at, pc.updated_at
`

// PartnerRepository implements domain.PartnerRepository using pgxpool.
type PartnerRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewPartnerRepository creates a new partner repository.
func NewPartnerRepository(pool *pgxpool.Pool) *PartnerRepository {
	return &PartnerRepository{pool: pool, timeout: 5 * time.Second}
}

// Create inserts a new partner record.
func (r *PartnerRepository) Create(ctx context.Context, partner *domain.Partner) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO partners (org_id, user_id, store_name, city, business_location, referral_code, commission_percentage, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		RETURNING id, created_at, updated_at
	`, partner.OrgID, partner.UserID, partner.StoreName, partner.City, partner.BusinessLocation, partner.ReferralCode, partner.CommissionPercentage,
	).Scan(&partner.ID, &partner.CreatedAt, &partner.UpdatedAt)
}

// GetByID fetches a partner record by id.
func (r *PartnerRepository) GetByID(ctx context.Context, partnerID uuid.UUID) (*domain.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.scanPartnerRow(r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, store_name, city, business_location, referral_code, commission_percentage, status, created_at, updated_at
		FROM partners
		WHERE id = $1
	`, partnerID))
}

// GetByUser fetches a partner record by org_id and user_id.
func (r *PartnerRepository) GetByUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.scanPartnerRow(r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, store_name, city, business_location, referral_code, commission_percentage, status, created_at, updated_at
		FROM partners
		WHERE org_id = $1 AND user_id = $2
	`, orgID, userID))
}

// GetByReferralCode fetches an active partner by permanent referral code.
func (r *PartnerRepository) GetByReferralCode(ctx context.Context, code string) (*domain.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.scanPartnerRow(r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, store_name, city, business_location, referral_code, commission_percentage, status, created_at, updated_at
		FROM partners
		WHERE referral_code = $1
	`, code))
}

// GetProfile fetches the partner profile with aggregated stats.
func (r *PartnerRepository) GetProfile(ctx context.Context, orgID, userID uuid.UUID) (*domain.PartnerProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	prof := &domain.PartnerProfile{}
	err := r.pool.QueryRow(ctx, `
		SELECT
			p.id, p.store_name, p.city, p.business_location, p.referral_code, p.commission_percentage, p.status,
			COALESCE(client_stats.total_clients, 0) AS total_clients,
			COALESCE(client_stats.active_clients, 0) AS active_clients,
			COALESCE(client_stats.plans_purchased, 0) AS plans_purchased,
			COALESCE(commission_stats.total_commission_earned_xof, 0) AS total_commission_earned_xof,
			COALESCE(commission_stats.total_commission_owed_xof, 0) AS total_commission_owed_xof,
			COALESCE(commission_stats.total_commission_paid_xof, 0) AS total_commission_paid_xof
		FROM partners p
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) AS total_clients,
				COUNT(*) FILTER (WHERE status = 'active') AS active_clients,
				COUNT(*) FILTER (WHERE status != 'invited') AS plans_purchased
			FROM partner_clients pc
			WHERE pc.partner_id = p.id
		) AS client_stats ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				SUM(commission_amount_xof) AS total_commission_earned_xof,
				SUM(commission_amount_xof) FILTER (WHERE status != 'paid') AS total_commission_owed_xof,
				SUM(commission_amount_xof) FILTER (WHERE status = 'paid') AS total_commission_paid_xof
			FROM partner_commissions comm
			WHERE comm.partner_id = p.id
		) AS commission_stats ON TRUE
		WHERE p.org_id = $1 AND p.user_id = $2
	`, orgID, userID).Scan(
		&prof.ID, &prof.StoreName, &prof.City, &prof.BusinessLocation, &prof.ReferralCode, &prof.CommissionPercentage, &prof.Status,
		&prof.TotalClients, &prof.ActiveClients, &prof.PlansPurchased,
		&prof.TotalCommissionEarnedXOF, &prof.TotalCommissionOwedXOF, &prof.TotalCommissionPaidXOF,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return prof, nil
}

// GetClientByLinkedUser returns the attributed partner client for a linked user.
func (r *PartnerRepository) GetClientByLinkedUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.PartnerClient, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.scanClientRow(r.pool.QueryRow(ctx, `
		SELECT `+partnerClientColumns+`
		FROM partner_clients
		WHERE org_id = $1 AND linked_user_id = $2
	`, orgID, userID))
}

// CreateClient inserts a new partner client record.
func (r *PartnerRepository) CreateClient(ctx context.Context, client *domain.PartnerClient) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO partner_clients (
			org_id, partner_id, linked_user_id, client_name, client_phone, plan_id, status,
			attribution_source, referral_code, referral_medium, attributed_at,
			invitation_token, invitation_expires_at, invitation_claimed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING `+partnerClientColumns+`
	`, client.OrgID, client.PartnerID, client.LinkedUserID, client.ClientName, client.ClientPhone, client.PlanID,
		client.Status, client.AttributionSource, client.ReferralCode, client.ReferralMedium, client.AttributedAt,
		client.InvitationToken, client.InvitationExpiresAt, client.InvitationClaimedAt,
	).Scan(
		&client.ID, &client.OrgID, &client.PartnerID, &client.LinkedUserID, &client.ClientName,
		&client.ClientPhone, &client.PlanID, &client.Status, &client.AttributionSource, &client.ReferralCode,
		&client.ReferralMedium, &client.AttributedAt, &client.InvitationToken, &client.InvitationExpiresAt,
		&client.InvitationClaimedAt, &client.InvitedAt, &client.CreatedAt, &client.UpdatedAt,
	)
}

// GetClientByID returns a partner client by id.
func (r *PartnerRepository) GetClientByID(ctx context.Context, clientID uuid.UUID) (*domain.PartnerClient, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.scanClientRow(r.pool.QueryRow(ctx, `
		SELECT `+partnerClientColumns+`
		FROM partner_clients
		WHERE id = $1
	`, clientID))
}

// GetClientByInvitationToken returns a partner client by invitation token.
func (r *PartnerRepository) GetClientByInvitationToken(ctx context.Context, token string) (*domain.PartnerClient, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.scanClientRow(r.pool.QueryRow(ctx, `
		SELECT `+partnerClientColumns+`
		FROM partner_clients
		WHERE invitation_token = $1
	`, token))
}

// GetInvitationDetailsByToken returns public invitation context for onboarding.
func (r *PartnerRepository) GetInvitationDetailsByToken(ctx context.Context, token string) (*domain.PartnerInvitationDetails, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	details := &domain.PartnerInvitationDetails{}
	err := r.pool.QueryRow(ctx, `
		SELECT
			pc.id,
			pc.partner_id,
			p.store_name,
			p.city,
			pc.client_name,
			pc.client_phone,
			pc.plan_id,
			pl.name_fr,
			pl.name_en,
			pc.status,
			pc.invitation_expires_at,
			pc.invitation_claimed_at,
			pc.linked_user_id
		FROM partner_clients pc
		JOIN partners p ON p.id = pc.partner_id
		LEFT JOIN plans pl ON pl.id = pc.plan_id
		WHERE pc.invitation_token = $1
	`, token).Scan(
		&details.ClientID,
		&details.PartnerID,
		&details.PartnerStoreName,
		&details.PartnerCity,
		&details.ClientName,
		&details.ClientPhone,
		&details.PlanID,
		&details.PlanNameFR,
		&details.PlanNameEN,
		&details.Status,
		&details.InvitationExpiresAt,
		&details.InvitationClaimedAt,
		&details.LinkedUserID,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return details, nil
}

// GetReferralDetailsByCode returns public partner context for reusable referral links.
func (r *PartnerRepository) GetReferralDetailsByCode(ctx context.Context, code string) (*domain.PartnerReferralDetails, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	details := &domain.PartnerReferralDetails{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, store_name, city, referral_code, status
		FROM partners
		WHERE referral_code = $1
	`, code).Scan(
		&details.PartnerID,
		&details.PartnerStoreName,
		&details.PartnerCity,
		&details.ReferralCode,
		&details.Status,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return details, nil
}

// ListClients returns all clients for a given partner.
func (r *PartnerRepository) ListClients(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]domain.PartnerClient, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+partnerClientColumnsQualified+`,
			(comm.id IS NOT NULL) AS has_generated_commission,
			comm.commission_amount_xof,
			comm.status,
			comm.commission_percentage,
			comm.created_at
		FROM partner_clients pc
		LEFT JOIN partner_commissions comm ON comm.partner_client_id = pc.id
		WHERE pc.partner_id = $1
		ORDER BY pc.invited_at DESC
		LIMIT $2 OFFSET $3
	`, partnerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []domain.PartnerClient
	for rows.Next() {
		var c domain.PartnerClient
		if err := rows.Scan(
			&c.ID,
			&c.OrgID,
			&c.PartnerID,
			&c.LinkedUserID,
			&c.ClientName,
			&c.ClientPhone,
			&c.PlanID,
			&c.Status,
			&c.AttributionSource,
			&c.ReferralCode,
			&c.ReferralMedium,
			&c.AttributedAt,
			&c.InvitationToken,
			&c.InvitationExpiresAt,
			&c.InvitationClaimedAt,
			&c.InvitedAt,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.HasGeneratedCommission,
			&c.CommissionAmountXOF,
			&c.CommissionStatus,
			&c.CommissionPercentage,
			&c.CommissionCreatedAt,
		); err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}
	if clients == nil {
		clients = []domain.PartnerClient{}
	}
	return clients, rows.Err()
}

// ClaimClientInvitation attaches a logged-in user to an invitation.
func (r *PartnerRepository) ClaimClientInvitation(ctx context.Context, clientID, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	commandTag, err := r.pool.Exec(ctx, `
		UPDATE partner_clients
		SET
			linked_user_id = CASE
				WHEN linked_user_id IS NULL THEN $2
				ELSE linked_user_id
			END,
			attributed_at = COALESCE(attributed_at, now()),
			invitation_claimed_at = COALESCE(invitation_claimed_at, now()),
			status = CASE
				WHEN status IN ('invited', 'draft', 'expired') THEN 'account_created'
				ELSE status
			END,
			updated_at = now()
		WHERE id = $1
		  AND (linked_user_id IS NULL OR linked_user_id = $2)
	`, clientID, userID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// RefreshClientInvitation rotates the invitation token and extends validity.
func (r *PartnerRepository) RefreshClientInvitation(ctx context.Context, clientID uuid.UUID, token string, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE partner_clients
		SET invitation_token = $2,
		    invitation_expires_at = $3,
		    status = CASE
				WHEN status IN ('draft', 'invited', 'expired') THEN 'invited'
				ELSE status
			END,
		    updated_at = now()
		WHERE id = $1
	`, clientID, token, expiresAt)
	return err
}

// UpdateClientStatus updates the status (and optionally plan_id) of a partner client.
func (r *PartnerRepository) UpdateClientStatus(ctx context.Context, clientID uuid.UUID, status string, planID *uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE partner_clients
		SET status = $2, plan_id = COALESCE($3, plan_id), updated_at = now()
		WHERE id = $1
	`, clientID, status, planID)
	return err
}

// UpdateClientStatusByLinkedUser moves the attributed partner client forward in the lifecycle.
func (r *PartnerRepository) UpdateClientStatusByLinkedUser(ctx context.Context, userID uuid.UUID, status string, planID *uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE partner_clients
		SET status = $2,
		    plan_id = COALESCE($3, plan_id),
		    updated_at = now()
		WHERE linked_user_id = $1
	`, userID, status, planID)
	return err
}

// CreateReferralVisit records a public partner referral landing event.
func (r *PartnerRepository) CreateReferralVisit(ctx context.Context, visit *domain.PartnerReferralVisit) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO partner_referral_visits (
			org_id, partner_id, referral_code, visitor_token, source_medium, visited_at
		)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6, now()))
		RETURNING id, created_at
	`, visit.OrgID, visit.PartnerID, visit.ReferralCode, visit.VisitorToken, visit.SourceMedium, visit.VisitedAt).Scan(
		&visit.ID,
		&visit.CreatedAt,
	)
}

// GetReferralMetrics aggregates reusable-link visits and conversion outcomes for a partner.
func (r *PartnerRepository) GetReferralMetrics(ctx context.Context, partnerID uuid.UUID) (*domain.PartnerReferralMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	metrics := &domain.PartnerReferralMetrics{}
	err := r.pool.QueryRow(ctx, `
		WITH visit_stats AS (
			SELECT
				COUNT(*)::int AS total_visits,
				COUNT(*) FILTER (WHERE source_medium = 'qr')::int AS qr_visits,
				COUNT(*) FILTER (WHERE source_medium = 'share')::int AS share_visits
			FROM partner_referral_visits
			WHERE partner_id = $1
		),
		signup_stats AS (
			SELECT
				COUNT(*) FILTER (WHERE attribution_source = 'partner_referral_link')::int AS total_signups,
				COUNT(*) FILTER (
					WHERE attribution_source = 'partner_referral_link'
					  AND status = 'payment_pending'
				)::int AS payment_pending_count,
				COUNT(*) FILTER (
					WHERE attribution_source = 'partner_referral_link'
					  AND status = 'active'
				)::int AS active_clients
			FROM partner_clients
			WHERE partner_id = $1
		)
		SELECT
			COALESCE(visit_stats.total_visits, 0),
			COALESCE(visit_stats.qr_visits, 0),
			COALESCE(visit_stats.share_visits, 0),
			COALESCE(signup_stats.total_signups, 0),
			COALESCE(signup_stats.payment_pending_count, 0),
			COALESCE(signup_stats.active_clients, 0)
		FROM visit_stats, signup_stats
	`, partnerID).Scan(
		&metrics.TotalVisits,
		&metrics.QRVisits,
		&metrics.ShareVisits,
		&metrics.TotalSignups,
		&metrics.PaymentPendingCount,
		&metrics.ActiveClients,
	)
	if err != nil {
		return nil, err
	}
	if metrics.TotalVisits > 0 {
		metrics.ConversionRate = float64(metrics.TotalSignups) * 100 / float64(metrics.TotalVisits)
	}
	return metrics, nil
}

// ListPlanBreakdown returns the chosen plan mix for a partner's referred clients.
func (r *PartnerRepository) ListPlanBreakdown(ctx context.Context, partnerID uuid.UUID, limit int) ([]domain.PartnerPlanBreakdown, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 10
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			pc.plan_id::text,
			pl.name_fr,
			pl.name_en,
			COUNT(*)::int AS count
		FROM partner_clients pc
		LEFT JOIN plans pl ON pl.id = pc.plan_id
		WHERE pc.partner_id = $1
		  AND pc.plan_id IS NOT NULL
		GROUP BY pc.plan_id, pl.name_fr, pl.name_en
		ORDER BY count DESC, pl.name_fr ASC NULLS LAST
		LIMIT $2
	`, partnerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.PartnerPlanBreakdown
	for rows.Next() {
		var item domain.PartnerPlanBreakdown
		if err := rows.Scan(&item.PlanID, &item.PlanNameFR, &item.PlanNameEN, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = []domain.PartnerPlanBreakdown{}
	}
	return items, rows.Err()
}

// CreateCommission inserts a commission record if it has not already been recorded.
func (r *PartnerRepository) CreateCommission(ctx context.Context, commission *domain.PartnerCommission) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	err := r.pool.QueryRow(ctx, `
		INSERT INTO partner_commissions (
			org_id, partner_id, partner_client_id, client_user_id, payment_id, plan_id,
			base_amount_xof, commission_percentage, commission_amount_xof, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT DO NOTHING
		RETURNING id, created_at, updated_at
	`, commission.OrgID, commission.PartnerID, commission.PartnerClientID, commission.ClientUserID, commission.PaymentID,
		commission.PlanID, commission.BaseAmountXOF, commission.CommissionPercentage, commission.CommissionAmountXOF, commission.Status,
	).Scan(&commission.ID, &commission.CreatedAt, &commission.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil
	}
	return err
}

// ListSales returns commission records with payment details for a partner.
func (r *PartnerRepository) ListSales(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]domain.PartnerSale, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			comm.id::text,
			comm.partner_client_id::text,
			comm.client_user_id::text,
			comm.payment_id::text,
			comm.plan_id::text,
			COALESCE(pc.client_name, u.full_name, '-') AS customer_name,
			pl.name_fr,
			pl.name_en,
			COALESCE(comm.base_amount_xof, 0),
			COALESCE(comm.commission_percentage, 0),
			comm.commission_amount_xof,
			comm.status,
			pay.paid_at,
			comm.created_at
		FROM partner_commissions comm
		LEFT JOIN payments pay ON pay.id = comm.payment_id
		LEFT JOIN users u ON u.id = comm.client_user_id
		LEFT JOIN partner_clients pc ON pc.id = comm.partner_client_id
		LEFT JOIN plans pl ON pl.id = comm.plan_id
		WHERE comm.partner_id = $1
		ORDER BY COALESCE(pay.paid_at, comm.created_at) DESC, comm.created_at DESC
		LIMIT $2 OFFSET $3
	`, partnerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sales []domain.PartnerSale
	for rows.Next() {
		var s domain.PartnerSale
		if err := rows.Scan(
			&s.ID, &s.PartnerClientID, &s.ClientUserID, &s.PaymentID, &s.PlanID,
			&s.CustomerName, &s.PlanNameFR, &s.PlanNameEN, &s.BaseAmountXOF,
			&s.CommissionPercentage, &s.CommissionAmountXOF, &s.Status, &s.PaidAt, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		sales = append(sales, s)
	}
	if sales == nil {
		sales = []domain.PartnerSale{}
	}
	return sales, rows.Err()
}

func (r *PartnerRepository) scanClientRow(row pgx.Row) (*domain.PartnerClient, error) {
	var client domain.PartnerClient
	err := row.Scan(
		&client.ID,
		&client.OrgID,
		&client.PartnerID,
		&client.LinkedUserID,
		&client.ClientName,
		&client.ClientPhone,
		&client.PlanID,
		&client.Status,
		&client.AttributionSource,
		&client.ReferralCode,
		&client.ReferralMedium,
		&client.AttributedAt,
		&client.InvitationToken,
		&client.InvitationExpiresAt,
		&client.InvitationClaimedAt,
		&client.InvitedAt,
		&client.CreatedAt,
		&client.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &client, nil
}

func (r *PartnerRepository) scanClientRows(rows pgx.Rows) (*domain.PartnerClient, error) {
	var client domain.PartnerClient
	if err := rows.Scan(
		&client.ID,
		&client.OrgID,
		&client.PartnerID,
		&client.LinkedUserID,
		&client.ClientName,
		&client.ClientPhone,
		&client.PlanID,
		&client.Status,
		&client.AttributionSource,
		&client.ReferralCode,
		&client.ReferralMedium,
		&client.AttributedAt,
		&client.InvitationToken,
		&client.InvitationExpiresAt,
		&client.InvitationClaimedAt,
		&client.InvitedAt,
		&client.CreatedAt,
		&client.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &client, nil
}

// ListPayouts returns paid commission records for a partner.
func (r *PartnerRepository) ListPayouts(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]domain.PartnerPayout, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, commission_amount_xof, COALESCE(payout_method, ''), status, paid_at
		FROM partner_commissions
		WHERE partner_id = $1 AND status = 'paid' AND paid_at IS NOT NULL
		ORDER BY paid_at DESC
		LIMIT $2 OFFSET $3
	`, partnerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payouts []domain.PartnerPayout
	for rows.Next() {
		var p domain.PartnerPayout
		if err := rows.Scan(&p.ID, &p.AmountXOF, &p.PayoutMethod, &p.Status, &p.PaidAt); err != nil {
			return nil, err
		}
		payouts = append(payouts, p)
	}
	if payouts == nil {
		payouts = []domain.PartnerPayout{}
	}
	return payouts, rows.Err()
}

// ListAll returns all partners for the admin dashboard.
func (r *PartnerRepository) ListAll(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]domain.AdminPartner, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			p.id::text,
			p.store_name,
			u.full_name AS owner_name,
			p.city,
			p.business_location,
			p.referral_code,
			p.commission_percentage,
			COALESCE(client_stats.clients_count, 0) AS clients_count,
			COALESCE(client_stats.active_clients, 0) AS active_clients,
			COALESCE(referral_stats.referral_visits, 0) AS referral_visits,
			COALESCE(referral_stats.qr_referral_visits, 0) AS qr_referral_visits,
			COALESCE(referral_stats.referral_signups, 0) AS referral_signups,
			COALESCE(referral_stats.referral_activations, 0) AS referral_activations,
			COALESCE(commission_stats.total_commission_earned_xof, 0) AS total_commission_earned_xof,
			COALESCE(commission_stats.total_commission_owed_xof, 0) AS total_commission_owed_xof,
			COALESCE(commission_stats.total_commission_paid_xof, 0) AS total_commission_paid_xof,
			p.status,
			to_char(p.created_at, 'Mon YYYY') AS joined_at
		FROM partners p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) AS clients_count,
				COUNT(*) FILTER (WHERE status = 'active') AS active_clients
			FROM partner_clients pc
			WHERE pc.partner_id = p.id
		) AS client_stats ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				(SELECT COUNT(*) FROM partner_referral_visits prv WHERE prv.partner_id = p.id) AS referral_visits,
				(SELECT COUNT(*) FROM partner_referral_visits prv WHERE prv.partner_id = p.id AND prv.source_medium = 'qr') AS qr_referral_visits,
				(SELECT COUNT(*) FROM partner_clients pc WHERE pc.partner_id = p.id AND pc.attribution_source = 'partner_referral_link') AS referral_signups,
				(SELECT COUNT(*) FROM partner_clients pc WHERE pc.partner_id = p.id AND pc.attribution_source = 'partner_referral_link' AND pc.status = 'active') AS referral_activations
		) AS referral_stats ON TRUE
		LEFT JOIN LATERAL (
			SELECT
				SUM(commission_amount_xof) AS total_commission_earned_xof,
				SUM(commission_amount_xof) FILTER (WHERE status != 'paid') AS total_commission_owed_xof,
				SUM(commission_amount_xof) FILTER (WHERE status = 'paid') AS total_commission_paid_xof
			FROM partner_commissions comm
			WHERE comm.partner_id = p.id
		) AS commission_stats ON TRUE
		WHERE p.org_id = $1
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []domain.AdminPartner
	for rows.Next() {
		var ap domain.AdminPartner
		if err := rows.Scan(
			&ap.ID, &ap.StoreName, &ap.OwnerName, &ap.City, &ap.BusinessLocation, &ap.ReferralCode, &ap.CommissionPercentage,
			&ap.ClientsCount, &ap.ActiveClients, &ap.ReferralVisits, &ap.QRReferralVisits, &ap.ReferralSignups, &ap.ReferralActivations, &ap.TotalCommissionEarnedXOF,
			&ap.TotalCommissionOwedXOF, &ap.TotalCommissionPaidXOF, &ap.Status, &ap.JoinedAt,
		); err != nil {
			return nil, err
		}
		if ap.ReferralVisits > 0 {
			ap.ReferralConversionRate = float64(ap.ReferralSignups) * 100 / float64(ap.ReferralVisits)
		}
		partners = append(partners, ap)
	}
	if partners == nil {
		partners = []domain.AdminPartner{}
	}
	return partners, rows.Err()
}

// ListAdminCommissions returns line-item commission reporting for an admin-scoped partner.
func (r *PartnerRepository) ListAdminCommissions(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]domain.AdminPartnerCommission, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			comm.id::text,
			comm.partner_client_id::text,
			comm.client_user_id::text,
			comm.payment_id::text,
			comm.plan_id::text,
			COALESCE(pc.client_name, u.full_name, '-') AS customer_name,
			pl.name_fr,
			pl.name_en,
			COALESCE(comm.base_amount_xof, 0),
			COALESCE(comm.commission_percentage, 0),
			comm.commission_amount_xof,
			comm.status,
			comm.paid_at,
			comm.created_at
		FROM partner_commissions comm
		LEFT JOIN partner_clients pc ON pc.id = comm.partner_client_id
		LEFT JOIN users u ON u.id = comm.client_user_id
		LEFT JOIN plans pl ON pl.id = comm.plan_id
		WHERE comm.partner_id = $1
		ORDER BY comm.created_at DESC
		LIMIT $2 OFFSET $3
	`, partnerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commissions []domain.AdminPartnerCommission
	for rows.Next() {
		var commission domain.AdminPartnerCommission
		if err := rows.Scan(
			&commission.ID,
			&commission.PartnerClientID,
			&commission.ClientUserID,
			&commission.PaymentID,
			&commission.PlanID,
			&commission.CustomerName,
			&commission.PlanNameFR,
			&commission.PlanNameEN,
			&commission.BaseAmountXOF,
			&commission.CommissionPercentage,
			&commission.CommissionAmountXOF,
			&commission.Status,
			&commission.PaidAt,
			&commission.CreatedAt,
		); err != nil {
			return nil, err
		}
		commissions = append(commissions, commission)
	}
	if commissions == nil {
		commissions = []domain.AdminPartnerCommission{}
	}
	return commissions, rows.Err()
}

// ListAdminReferrals returns customer-level attribution reporting for a specific partner.
func (r *PartnerRepository) ListAdminReferrals(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]domain.AdminPartnerReferral, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			pc.id::text,
			pc.linked_user_id::text,
			pc.client_name,
			u.email,
			COALESCE(pc.client_phone, u.phone) AS customer_phone,
			pc.attribution_source,
			pc.referral_code,
			pc.referral_medium,
			pc.attributed_at,
			pc.plan_id::text,
			pl.name_fr,
			pl.name_en,
			pc.status,
			sub.status::text AS subscription_status,
			pay.status::text AS payment_status,
			(comm.id IS NOT NULL) AS has_generated_commission,
			comm.commission_amount_xof,
			comm.status
		FROM partner_clients pc
		LEFT JOIN users u ON u.id = pc.linked_user_id
		LEFT JOIN plans pl ON pl.id = pc.plan_id
		LEFT JOIN LATERAL (
			SELECT s.status
			FROM subscriptions s
			WHERE s.user_id = pc.linked_user_id
			ORDER BY s.created_at DESC
			LIMIT 1
		) sub ON TRUE
		LEFT JOIN LATERAL (
			SELECT p.status
			FROM payments p
			WHERE p.user_id = pc.linked_user_id
			ORDER BY COALESCE(p.paid_at, p.created_at) DESC
			LIMIT 1
		) pay ON TRUE
		LEFT JOIN partner_commissions comm ON comm.partner_client_id = pc.id
		WHERE pc.partner_id = $1
		ORDER BY COALESCE(pc.attributed_at, pc.invited_at, pc.created_at) DESC
		LIMIT $2 OFFSET $3
	`, partnerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.AdminPartnerReferral
	for rows.Next() {
		var item domain.AdminPartnerReferral
		if err := rows.Scan(
			&item.PartnerClientID,
			&item.ClientUserID,
			&item.CustomerName,
			&item.CustomerEmail,
			&item.CustomerPhone,
			&item.AttributionSource,
			&item.ReferralCode,
			&item.ReferralMedium,
			&item.AttributedAt,
			&item.PlanID,
			&item.PlanNameFR,
			&item.PlanNameEN,
			&item.ClientStatus,
			&item.SubscriptionStatus,
			&item.PaymentStatus,
			&item.HasGeneratedCommission,
			&item.CommissionAmountXOF,
			&item.CommissionStatus,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = []domain.AdminPartnerReferral{}
	}
	return items, rows.Err()
}

func (r *PartnerRepository) scanPartnerRow(row pgx.Row) (*domain.Partner, error) {
	var partner domain.Partner
	err := row.Scan(
		&partner.ID,
		&partner.OrgID,
		&partner.UserID,
		&partner.StoreName,
		&partner.City,
		&partner.BusinessLocation,
		&partner.ReferralCode,
		&partner.CommissionPercentage,
		&partner.Status,
		&partner.CreatedAt,
		&partner.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &partner, nil
}
