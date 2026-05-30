package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		RevenueByProvider: []domain.ProviderRevenue{},
	}

	var revenueByProvider json.RawMessage
	if err := r.pool.QueryRow(ctx, `
		WITH revenue_by_provider AS (
			-- Group by (provider, market) so currencies are NEVER summed:
			-- 1499 cents (USD) + 3000 XOF would produce a nonsense total.
			-- The frontend renders one card per row using CurrencyForMarket.
			SELECT COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'provider', provider,
						'market', market,
						'amount_minor', amount_minor
					)
					ORDER BY market, provider
				),
				'[]'::jsonb
			) AS payload
			FROM (
				SELECT provider, market, SUM(amount_minor)::int AS amount_minor
				FROM payments
				WHERE org_id = $1
				  AND status = 'completed'
				GROUP BY provider, market
			) grouped_revenue
		)
		SELECT
			(SELECT COUNT(*)::int FROM subscriptions WHERE org_id = $1 AND status = 'active'),
			(
				SELECT COALESCE(SUM(amount_minor), 0)::int
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
				  -- Mirror the Customers list filter — see ListCustomers for
				  -- the rationale. Keep these in sync.
				  AND role IN ('member', 'partner', 'commercial')
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
			partner_attr.partner_store_name,
			partner_attr.partner_referral_code,
			partner_attr.partner_attribution_source,
			partner_attr.partner_attributed_at,
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
		LEFT JOIN LATERAL (
			SELECT
				p.store_name AS partner_store_name,
				pc.referral_code AS partner_referral_code,
				pc.attribution_source AS partner_attribution_source,
				pc.attributed_at AS partner_attributed_at
			FROM partner_clients pc
			JOIN partners p ON p.id = pc.partner_id
			WHERE pc.org_id = u.org_id
			  AND pc.linked_user_id = u.id
			ORDER BY COALESCE(pc.attributed_at, pc.invited_at, pc.created_at) DESC
			LIMIT 1
		) AS partner_attr ON TRUE
		LEFT JOIN subscriptions s ON s.user_id = u.id AND s.org_id = u.org_id
		LEFT JOIN plans p ON p.id = s.plan_id
		LEFT JOIN devices d ON d.id = s.device_id AND d.org_id = s.org_id AND d.deleted_at IS NULL
		WHERE u.org_id = $1
		  AND u.deleted_at IS NULL
		  AND ($2 = '' OR lower(u.full_name) LIKE '%' || lower($2) || '%' OR lower(u.email) LIKE '%' || lower($2) || '%')
		  -- "Customer" = anyone who could buy a plan. Partners and commercials
		  -- often subscribe to a plan for their own phone too; excluding them
		  -- here means an admin would never see those subscriptions on the
		  -- Customers tab even though they're paying for our service. Internal
		  -- staff (admin/employee) is the only group that shouldn't surface
		  -- here — they have their own tabs.
		  AND u.role IN ('member', 'partner', 'commercial')
		GROUP BY
			u.id,
			u.full_name,
			u.phone,
			u.email,
			u.created_at,
			partner_attr.partner_store_name,
			partner_attr.partner_referral_code,
			partner_attr.partner_attribution_source,
			partner_attr.partner_attributed_at
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
			&c.PartnerStoreName,
			&c.PartnerReferralCode,
			&c.PartnerAttributionSource,
			&c.PartnerAttributedAt,
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
			pay.amount_minor,
			pay.market,
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
		if err := rows.Scan(&p.ID, &p.CustomerName, &p.PlanNameFR, &p.PlanNameEN, &p.AmountMinor, &p.Market, &p.Provider, &p.PaymentMethod, &p.Status, &p.PaidAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	if payments == nil {
		payments = []domain.AdminPayment{}
	}
	return payments, rows.Err()
}

// ListEmployees returns admin employee summaries with workload context.
func (r *AdminRepository) ListEmployees(
	ctx context.Context,
	orgID uuid.UUID,
	search string,
	status *domain.EmployeeAccountStatus,
	sort string,
	limit,
	offset int,
) ([]domain.AdminEmployeeListItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	sortKey := "recent_activity"
	if strings.EqualFold(sort, "joined") {
		sortKey = "joined"
	}

	args := []any{orgID, strings.TrimSpace(search)}
	statusIndex := 0
	if status != nil {
		args = append(args, *status)
		statusIndex = len(args)
	}
	args = append(args, limit, offset)
	limitIndex := len(args) - 1
	offsetIndex := len(args)

	query := `
		WITH follow_up_owners AS (
			SELECT
				COALESCE(f.updated_by, f.created_by) AS owner_id,
				f.entity_type,
				f.entity_id,
				f.updated_at
			FROM operational_follow_ups f
			WHERE f.org_id = $1
			  AND f.status != 'resolved'
			  AND COALESCE(f.updated_by, f.created_by) IS NOT NULL
		),
		client_follows AS (
			SELECT owner_id, COUNT(DISTINCT client_id)::int AS clients_followed_count
			FROM (
				SELECT owner_id, entity_id AS client_id
				FROM follow_up_owners
				WHERE entity_type = 'client'

				UNION ALL

				SELECT fo.owner_id, s.user_id AS client_id
				FROM follow_up_owners fo
				JOIN subscriptions s
				  ON fo.entity_type = 'subscription'
				 AND s.id = fo.entity_id

				UNION ALL

				SELECT fo.owner_id, c.user_id AS client_id
				FROM follow_up_owners fo
				JOIN claims c
				  ON fo.entity_type = 'claim'
				 AND c.id = fo.entity_id
			) mapped_clients
			GROUP BY owner_id
		),
		claim_counts AS (
			SELECT fo.owner_id, COUNT(*)::int AS active_claims_count
			FROM follow_up_owners fo
			JOIN claims c
			  ON fo.entity_type = 'claim'
			 AND c.id = fo.entity_id
			WHERE c.status IN ('pending', 'review')
			GROUP BY fo.owner_id
		),
		repair_counts AS (
			SELECT fo.owner_id, COUNT(*)::int AS active_repairs_count
			FROM follow_up_owners fo
			JOIN repair_bookings rb
			  ON fo.entity_type = 'repair'
			 AND rb.id = fo.entity_id
			WHERE rb.status IN ('pending', 'accepted', 'scheduled', 'in_progress')
			GROUP BY fo.owner_id
		),
		follow_up_counts AS (
			SELECT owner_id, COUNT(*)::int AS open_follow_ups_count, MAX(updated_at) AS last_follow_up_at
			FROM follow_up_owners
			GROUP BY owner_id
		),
		note_activity AS (
			SELECT created_by AS owner_id, MAX(created_at) AS last_note_at
			FROM operational_notes
			WHERE org_id = $1
			GROUP BY created_by
		),
		last_login AS (
			SELECT "userId" AS better_auth_id, MAX("createdAt") AS last_login_at
			FROM "session"
			GROUP BY "userId"
		)
		SELECT
			u.id::text,
			u.better_auth_id,
			u.full_name,
			u.email,
			u.phone,
			u.role,
			COALESCE(ep.status, 'active') AS status,
			ep.suspended_reason,
			u.created_at,
			COALESCE(cf.clients_followed_count, 0),
			COALESCE(cc.active_claims_count, 0),
			COALESCE(rc.active_repairs_count, 0),
			COALESCE(fc.open_follow_ups_count, 0),
			NULLIF(
				GREATEST(
					COALESCE(fc.last_follow_up_at, 'epoch'::timestamptz),
					COALESCE(na.last_note_at, 'epoch'::timestamptz)
				),
				'epoch'::timestamptz
			) AS last_activity_at,
			ll.last_login_at
		FROM users u
		LEFT JOIN employee_profiles ep
		  ON ep.user_id = u.id
		 AND ep.org_id = u.org_id
		LEFT JOIN client_follows cf ON cf.owner_id = u.id
		LEFT JOIN claim_counts cc ON cc.owner_id = u.id
		LEFT JOIN repair_counts rc ON rc.owner_id = u.id
		LEFT JOIN follow_up_counts fc ON fc.owner_id = u.id
		LEFT JOIN note_activity na ON na.owner_id = u.id
		LEFT JOIN last_login ll ON ll.better_auth_id = u.better_auth_id
		WHERE u.org_id = $1
		  AND u.deleted_at IS NULL
		  AND (u.role = 'employee' OR ep.user_id IS NOT NULL)
		  AND (
			$2 = ''
			OR lower(u.full_name) LIKE '%' || lower($2) || '%'
			OR lower(u.email) LIKE '%' || lower($2) || '%'
			OR COALESCE(u.phone, '') LIKE '%' || $2 || '%'
		  )
	`
	if status != nil {
		query += ` AND COALESCE(ep.status, 'active') = $` + strconv.Itoa(statusIndex)
	}

	switch sortKey {
	case "joined":
		query += `
			ORDER BY u.created_at DESC, u.full_name ASC
		`
	default:
		query += `
			ORDER BY last_activity_at DESC NULLS LAST, u.created_at DESC, u.full_name ASC
		`
	}

	query += `
		LIMIT $` + strconv.Itoa(limitIndex) + ` OFFSET $` + strconv.Itoa(offsetIndex)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AdminEmployeeListItem, 0)
	for rows.Next() {
		var item domain.AdminEmployeeListItem
		if err := rows.Scan(
			&item.ID,
			&item.BetterAuthID,
			&item.FullName,
			&item.Email,
			&item.Phone,
			&item.Role,
			&item.Status,
			&item.SuspendedReason,
			&item.JoinedAt,
			&item.Workload.ClientsFollowedCount,
			&item.Workload.ActiveClaimsCount,
			&item.Workload.ActiveRepairsCount,
			&item.Workload.OpenFollowUpsCount,
			&item.Workload.LastActivityAt,
			&item.Workload.LastLoginAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if items == nil {
		return []domain.AdminEmployeeListItem{}, nil
	}
	return items, rows.Err()
}

// GetEmployee returns a detailed employee management payload.
func (r *AdminRepository) GetEmployee(ctx context.Context, orgID, userID uuid.UUID) (*domain.AdminEmployeeDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	item := &domain.AdminEmployeeDetail{
		RecentActivity:    []domain.AdminEmployeeActivityItem{},
		PermissionSummary: []string{},
	}

	err := r.pool.QueryRow(ctx, `
		WITH follow_up_owners AS (
			SELECT
				COALESCE(f.updated_by, f.created_by) AS owner_id,
				f.entity_type,
				f.entity_id,
				f.updated_at
			FROM operational_follow_ups f
			WHERE f.org_id = $1
			  AND f.status != 'resolved'
			  AND COALESCE(f.updated_by, f.created_by) = $2
		),
		client_follows AS (
			SELECT COUNT(DISTINCT client_id)::int AS clients_followed_count
			FROM (
				SELECT entity_id AS client_id
				FROM follow_up_owners
				WHERE entity_type = 'client'

				UNION ALL

				SELECT s.user_id AS client_id
				FROM follow_up_owners fo
				JOIN subscriptions s
				  ON fo.entity_type = 'subscription'
				 AND s.id = fo.entity_id

				UNION ALL

				SELECT c.user_id AS client_id
				FROM follow_up_owners fo
				JOIN claims c
				  ON fo.entity_type = 'claim'
				 AND c.id = fo.entity_id
			) mapped_clients
		),
		claim_counts AS (
			SELECT COUNT(*)::int AS active_claims_count
			FROM follow_up_owners fo
			JOIN claims c
			  ON fo.entity_type = 'claim'
			 AND c.id = fo.entity_id
			WHERE c.status IN ('pending', 'review')
		),
		repair_counts AS (
			SELECT COUNT(*)::int AS active_repairs_count
			FROM follow_up_owners fo
			JOIN repair_bookings rb
			  ON fo.entity_type = 'repair'
			 AND rb.id = fo.entity_id
			WHERE rb.status IN ('pending', 'accepted', 'scheduled', 'in_progress')
		),
		follow_up_counts AS (
			SELECT COUNT(*)::int AS open_follow_ups_count, MAX(updated_at) AS last_follow_up_at
			FROM follow_up_owners
		),
		note_activity AS (
			SELECT MAX(created_at) AS last_note_at
			FROM operational_notes
			WHERE org_id = $1
			  AND created_by = $2
		),
		last_login AS (
			SELECT MAX(s."createdAt") AS last_login_at
			FROM users u
			LEFT JOIN "session" s ON s."userId" = u.better_auth_id
			WHERE u.id = $2
		)
		SELECT
			u.id::text,
			u.better_auth_id,
			u.full_name,
			u.email,
			u.phone,
			u.role,
			COALESCE(ep.status, 'active') AS status,
			ep.suspended_reason,
			u.created_at,
			u.updated_at,
			COALESCE((SELECT clients_followed_count FROM client_follows), 0),
			COALESCE((SELECT active_claims_count FROM claim_counts), 0),
			COALESCE((SELECT active_repairs_count FROM repair_counts), 0),
			COALESCE((SELECT open_follow_ups_count FROM follow_up_counts), 0),
			NULLIF(
				GREATEST(
					COALESCE((SELECT last_follow_up_at FROM follow_up_counts), 'epoch'::timestamptz),
					COALESCE((SELECT last_note_at FROM note_activity), 'epoch'::timestamptz)
				),
				'epoch'::timestamptz
			) AS last_activity_at,
			(SELECT last_login_at FROM last_login)
		FROM users u
		LEFT JOIN employee_profiles ep
		  ON ep.user_id = u.id
		 AND ep.org_id = u.org_id
		WHERE u.org_id = $1
		  AND u.id = $2
		  AND (u.role = 'employee' OR ep.user_id IS NOT NULL)
		  AND u.deleted_at IS NULL
	`, orgID, userID).Scan(
		&item.ID,
		&item.BetterAuthID,
		&item.FullName,
		&item.Email,
		&item.Phone,
		&item.Role,
		&item.Status,
		&item.SuspendedReason,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Workload.ClientsFollowedCount,
		&item.Workload.ActiveClaimsCount,
		&item.Workload.ActiveRepairsCount,
		&item.Workload.OpenFollowUpsCount,
		&item.Workload.LastActivityAt,
		&item.Workload.LastLoginAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	item.WorkspaceAccess = item.Status == domain.EmployeeAccountStatusActive
	item.PermissionSummary = employeePermissionSummary()

	activityRows, err := r.pool.Query(ctx, `
		SELECT kind, entity_type, entity_id, description, occurred_at
		FROM (
			SELECT
				'follow_up' AS kind,
				f.entity_type,
				f.entity_id::text AS entity_id,
				COALESCE(f.reason, f.next_action, 'Operational follow-up updated') AS description,
				f.updated_at AS occurred_at
			FROM operational_follow_ups f
			WHERE f.org_id = $1
			  AND COALESCE(f.updated_by, f.created_by) = $2

			UNION ALL

			SELECT
				'note' AS kind,
				n.entity_type,
				n.entity_id::text AS entity_id,
				LEFT(n.body, 240) AS description,
				n.created_at AS occurred_at
			FROM operational_notes n
			WHERE n.org_id = $1
			  AND n.created_by = $2
		) activity
		ORDER BY occurred_at DESC
		LIMIT 10
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer activityRows.Close()

	for activityRows.Next() {
		var activity domain.AdminEmployeeActivityItem
		if err := activityRows.Scan(
			&activity.Kind,
			&activity.EntityType,
			&activity.EntityID,
			&activity.Description,
			&activity.OccurredAt,
		); err != nil {
			return nil, err
		}
		item.RecentActivity = append(item.RecentActivity, activity)
	}
	if err := activityRows.Err(); err != nil {
		return nil, err
	}

	return item, nil
}

// UpdateEmployeeStatus updates the employee profile status for a SafePhone employee user.
func (r *AdminRepository) UpdateEmployeeStatus(
	ctx context.Context,
	orgID, userID, updatedBy uuid.UUID,
	status domain.EmployeeAccountStatus,
	suspendedReason *string,
) (*domain.EmployeeProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var profile domain.EmployeeProfile
	err := r.pool.QueryRow(ctx, `
		INSERT INTO employee_profiles (
			user_id,
			org_id,
			status,
			suspended_reason,
			created_by,
			updated_by
		)
		SELECT
			u.id,
			u.org_id,
			$4::varchar(20),
			CASE WHEN $4::text = 'suspended' THEN $5::text ELSE NULL END,
			$3,
			$3
		FROM users u
		WHERE u.id = $2
		  AND u.org_id = $1
		  AND (
			u.role = 'employee'
			OR EXISTS (
				SELECT 1
				FROM employee_profiles existing_ep
				WHERE existing_ep.user_id = u.id
				  AND existing_ep.org_id = u.org_id
			)
		  )
		  AND u.deleted_at IS NULL
		ON CONFLICT (user_id) DO UPDATE
		SET status = EXCLUDED.status,
			suspended_reason = EXCLUDED.suspended_reason,
			updated_by = EXCLUDED.updated_by,
			updated_at = now()
		RETURNING user_id, org_id, status, suspended_reason, created_by, updated_by, created_at, updated_at
	`, orgID, userID, updatedBy, status, suspendedReason).Scan(
		&profile.UserID,
		&profile.OrgID,
		&profile.Status,
		&profile.SuspendedReason,
		&profile.CreatedBy,
		&profile.UpdatedBy,
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

func employeePermissionSummary() []string {
	return []string{
		"Employee workspace access",
		"Client follow-up",
		"Claim processing",
		"Repair management",
		"Payment follow-up",
	}
}
