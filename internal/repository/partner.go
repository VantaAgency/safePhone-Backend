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
	invitation_token, invitation_expires_at, invitation_claimed_at, invited_at, created_at, updated_at
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
		INSERT INTO partners (org_id, user_id, store_name, city, commission_rate, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
		RETURNING id, created_at, updated_at
	`, partner.OrgID, partner.UserID, partner.StoreName, partner.City, partner.CommissionRate,
	).Scan(&partner.ID, &partner.CreatedAt, &partner.UpdatedAt)
}

// GetByUser fetches a partner record by org_id and user_id.
func (r *PartnerRepository) GetByUser(ctx context.Context, orgID, userID uuid.UUID) (*domain.Partner, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	p := &domain.Partner{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, user_id, store_name, city, commission_rate, status, created_at, updated_at
		FROM partners
		WHERE org_id = $1 AND user_id = $2
	`, orgID, userID).Scan(
		&p.ID, &p.OrgID, &p.UserID, &p.StoreName, &p.City,
		&p.CommissionRate, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetProfile fetches the partner profile with aggregated stats.
func (r *PartnerRepository) GetProfile(ctx context.Context, orgID, userID uuid.UUID) (*domain.PartnerProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	prof := &domain.PartnerProfile{}
	err := r.pool.QueryRow(ctx, `
		SELECT
			p.id, p.store_name, p.city, p.commission_rate, p.status,
			COUNT(DISTINCT pc.id) AS total_clients,
			COUNT(DISTINCT pc.id) FILTER (WHERE pc.status = 'active') AS active_clients,
			COUNT(DISTINCT pc.id) FILTER (WHERE pc.status != 'invited') AS plans_purchased,
			COALESCE(SUM(comm.amount_xof) FILTER (
				WHERE comm.status = 'paid' AND comm.created_at >= date_trunc('month', now())
			), 0) AS month_commission_xof
		FROM partners p
		LEFT JOIN partner_clients pc ON pc.partner_id = p.id
		LEFT JOIN partner_commissions comm ON comm.partner_id = p.id
		WHERE p.org_id = $1 AND p.user_id = $2
		GROUP BY p.id
	`, orgID, userID).Scan(
		&prof.ID, &prof.StoreName, &prof.City, &prof.CommissionRate, &prof.Status,
		&prof.TotalClients, &prof.ActiveClients, &prof.PlansPurchased, &prof.MonthCommissionXOF,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return prof, nil
}

// CreateClient inserts a new partner client record.
func (r *PartnerRepository) CreateClient(ctx context.Context, client *domain.PartnerClient) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO partner_clients (
			org_id, partner_id, linked_user_id, client_name, client_phone, plan_id, status,
			invitation_token, invitation_expires_at, invitation_claimed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+partnerClientColumns+`
	`, client.OrgID, client.PartnerID, client.LinkedUserID, client.ClientName, client.ClientPhone, client.PlanID,
		client.Status, client.InvitationToken, client.InvitationExpiresAt, client.InvitationClaimedAt,
	).Scan(
		&client.ID, &client.OrgID, &client.PartnerID, &client.LinkedUserID, &client.ClientName,
		&client.ClientPhone, &client.PlanID, &client.Status, &client.InvitationToken,
		&client.InvitationExpiresAt, &client.InvitationClaimedAt, &client.InvitedAt, &client.CreatedAt, &client.UpdatedAt,
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

// ListClients returns all clients for a given partner.
func (r *PartnerRepository) ListClients(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]domain.PartnerClient, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+partnerClientColumns+`
		FROM partner_clients
		WHERE partner_id = $1
		ORDER BY invited_at DESC
		LIMIT $2 OFFSET $3
	`, partnerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []domain.PartnerClient
	for rows.Next() {
		c, err := r.scanClientRows(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, *c)
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

// UpdateLatestClientStatusByLinkedUser moves the latest linked invitation forward in the lifecycle.
func (r *PartnerRepository) UpdateLatestClientStatusByLinkedUser(ctx context.Context, userID uuid.UUID, status string, planID *uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		WITH latest_client AS (
			SELECT id
			FROM partner_clients
			WHERE linked_user_id = $1
			ORDER BY invited_at DESC
			LIMIT 1
		)
		UPDATE partner_clients pc
		SET status = $2,
		    plan_id = COALESCE($3, pc.plan_id),
		    updated_at = now()
		FROM latest_client lc
		WHERE pc.id = lc.id
	`, userID, status, planID)
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
			u.full_name AS customer_name,
			pl.name_fr,
			pl.name_en,
			pay.amount_xof,
			comm.amount_xof AS commission_xof,
			pay.paid_at AS date
		FROM partner_commissions comm
		JOIN payments pay ON pay.id = comm.payment_id
		JOIN users u ON u.id = pay.user_id
		JOIN subscriptions sub ON sub.id = pay.subscription_id
		JOIN plans pl ON pl.id = sub.plan_id
		WHERE comm.partner_id = $1
		ORDER BY pay.paid_at DESC
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
			&s.ID, &s.CustomerName, &s.PlanNameFR, &s.PlanNameEN,
			&s.AmountXOF, &s.CommissionXOF, &s.Date,
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
		SELECT id::text, amount_xof, payout_method, status, paid_at
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
			COUNT(DISTINCT pc.id) AS clients_count,
			COUNT(DISTINCT pc.id) FILTER (WHERE pc.status = 'active') AS active_clients,
			COALESCE(SUM(comm.amount_xof) FILTER (
				WHERE comm.status = 'paid' AND comm.created_at >= date_trunc('month', now())
			), 0) AS commission_this_month,
			p.status,
			to_char(p.created_at, 'Mon YYYY') AS joined_at
		FROM partners p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN partner_clients pc ON pc.partner_id = p.id
		LEFT JOIN partner_commissions comm ON comm.partner_id = p.id
		WHERE p.org_id = $1
		GROUP BY p.id, u.full_name
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
			&ap.ID, &ap.StoreName, &ap.OwnerName, &ap.City,
			&ap.ClientsCount, &ap.ActiveClients, &ap.CommissionThisMonth,
			&ap.Status, &ap.JoinedAt,
		); err != nil {
			return nil, err
		}
		partners = append(partners, ap)
	}
	if partners == nil {
		partners = []domain.AdminPartner{}
	}
	return partners, rows.Err()
}
