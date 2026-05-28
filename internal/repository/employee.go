package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

const operationalFollowUpColumns = `
	id, org_id, entity_type, entity_id, reason, status, next_action, last_contact_at,
	created_by, updated_by, created_at, updated_at
`

// EmployeeRepository implements domain.EmployeeRepository using pgxpool.
type EmployeeRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewEmployeeRepository creates a new employee repository.
func NewEmployeeRepository(pool *pgxpool.Pool) *EmployeeRepository {
	return &EmployeeRepository{pool: pool, timeout: 8 * time.Second}
}

// GetOverviewMetrics returns employee dashboard counters.
func (r *EmployeeRepository) GetOverviewMetrics(ctx context.Context, orgID uuid.UUID) (*domain.EmployeeOverviewMetrics, error) {
	if err := expireEndedSubscriptions(ctx, r.pool, r.timeout, &orgID, nil, nil, nil); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	metrics := &domain.EmployeeOverviewMetrics{}
	err := r.pool.QueryRow(ctx, `
		WITH latest_subscriptions AS (
			SELECT DISTINCT ON (s.user_id, s.device_id)
				s.id,
				s.org_id,
				s.user_id,
				s.device_id,
				s.status
			FROM subscriptions s
			WHERE s.org_id = $1
			ORDER BY s.user_id, s.device_id, s.created_at DESC
		),
		latest_payments AS (
			SELECT DISTINCT ON (p.subscription_id)
				p.subscription_id,
				p.status
			FROM payments p
			WHERE p.org_id = $1
			ORDER BY p.subscription_id, p.created_at DESC
		)
		SELECT
			(
				SELECT COUNT(*)::int
				FROM latest_subscriptions ls
				LEFT JOIN latest_payments lp ON lp.subscription_id = ls.id
				WHERE ls.status = 'pending'
				   OR lp.status IN ('pending', 'failed', 'cancelled', 'expired')
			),
			(SELECT COUNT(*)::int FROM latest_payments WHERE status = 'pending'),
			(SELECT COUNT(*)::int FROM latest_payments WHERE status IN ('failed', 'cancelled', 'expired')),
			(
				SELECT COUNT(DISTINCT u.id)::int
				FROM users u
				WHERE u.org_id = $1
				  AND u.deleted_at IS NULL
				  AND u.role = 'member'
				  AND (
					EXISTS (
						SELECT 1
						FROM operational_follow_ups f
						WHERE f.org_id = u.org_id
						  AND f.entity_type = 'client'
						  AND f.entity_id = u.id
						  AND f.status != 'resolved'
					)
					OR EXISTS (
						SELECT 1
						FROM latest_subscriptions ls
						JOIN devices d ON d.id = ls.device_id AND d.deleted_at IS NULL
						LEFT JOIN latest_payments lp ON lp.subscription_id = ls.id
						WHERE ls.user_id = u.id
						  AND (
							ls.status = 'pending'
							OR lp.status IN ('pending', 'failed', 'cancelled', 'expired')
							OR (
								(ls.status = 'active' OR lp.status = 'completed')
								AND d.device_type = 'smartphone'
								AND NULLIF(TRIM(COALESCE(d.imei, '')), '') IS NULL
							)
						  )
					)
					OR EXISTS (
						SELECT 1
						FROM claims c
						WHERE c.org_id = u.org_id
						  AND c.user_id = u.id
						  AND c.status IN ('pending', 'review')
					)
					OR EXISTS (
						SELECT 1
						FROM repair_bookings rb
						WHERE rb.org_id = u.org_id
						  AND rb.user_id = u.id
						  AND rb.status IN ('pending', 'accepted', 'scheduled', 'in_progress')
					)
				  )
			),
			(
				SELECT COUNT(*)::int
				FROM claims
				WHERE org_id = $1
				  AND status IN ('pending', 'review')
			),
			(
				SELECT COUNT(*)::int
				FROM repair_bookings
				WHERE org_id = $1
				  AND status IN ('accepted', 'scheduled', 'in_progress')
			),
			(
				SELECT COUNT(*)::int
				FROM repair_bookings
				WHERE org_id = $1
				  AND status IN ('scheduled', 'in_progress')
				  AND scheduled_date IS NOT NULL
				  AND scheduled_date < CURRENT_DATE
			),
			(
				SELECT COUNT(*)::int
				FROM latest_subscriptions ls
				JOIN devices d ON d.id = ls.device_id AND d.deleted_at IS NULL
				LEFT JOIN latest_payments lp ON lp.subscription_id = ls.id
				WHERE (ls.status = 'active' OR (ls.status = 'pending' AND lp.status = 'completed'))
				  AND d.device_type = 'smartphone'
				  AND NULLIF(TRIM(COALESCE(d.imei, '')), '') IS NULL
			),
			(
				SELECT COUNT(*)::int
				FROM devices d
				JOIN users u ON u.id = d.user_id
				WHERE d.org_id = $1
				  AND d.deleted_at IS NULL
				  AND u.deleted_at IS NULL
				  AND u.role = 'member'
				  AND d.device_type = 'smartphone'
				  AND NULLIF(TRIM(COALESCE(d.imei, '')), '') IS NULL
			)
	`, orgID).Scan(
		&metrics.UnpaidSubscriptionsCount,
		&metrics.PendingPaymentsCount,
		&metrics.FailedPaymentsCount,
		&metrics.ClientsNeedingFollowUpCount,
		&metrics.PendingClaimsCount,
		&metrics.RepairsInProgressCount,
		&metrics.OverdueRepairsCount,
		&metrics.PendingActivationCount,
		&metrics.MissingIMEICount,
	)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// ListClients returns employee-oriented client summaries.
func (r *EmployeeRepository) ListClients(ctx context.Context, orgID uuid.UUID, search string, limit, offset int) ([]domain.EmployeeClientListItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			u.id,
			u.full_name,
			u.email,
			u.phone,
			(
				SELECT COUNT(*)::int
				FROM devices d
				WHERE d.org_id = u.org_id
				  AND d.user_id = u.id
				  AND d.deleted_at IS NULL
			) AS device_count,
			(
				SELECT COUNT(*)::int
				FROM subscriptions s
				WHERE s.org_id = u.org_id
				  AND s.user_id = u.id
				  AND s.status = 'active'
			) AS active_subscription_count,
			(
				SELECT COUNT(*)::int
				FROM devices d
				WHERE d.org_id = u.org_id
				  AND d.user_id = u.id
				  AND d.deleted_at IS NULL
				  AND d.device_type = 'smartphone'
				  AND NULLIF(TRIM(COALESCE(d.imei, '')), '') IS NULL
			) AS missing_imei_count,
			(
				SELECT COUNT(*)::int
				FROM claims c
				WHERE c.org_id = u.org_id
				  AND c.user_id = u.id
				  AND c.status IN ('pending', 'review')
			) AS pending_claims_count,
			(
				SELECT COUNT(*)::int
				FROM repair_bookings rb
				WHERE rb.org_id = u.org_id
				  AND rb.user_id = u.id
				  AND rb.status IN ('pending', 'accepted', 'scheduled', 'in_progress')
			) AS open_repairs_count,
			latest_device.device_type,
			latest_device.imei,
			latest_device.status,
			latest_sub.status,
			latest_payment.status,
			partner.store_name,
			(
				EXISTS (
					SELECT 1
					FROM operational_follow_ups f
					WHERE f.org_id = u.org_id
					  AND f.entity_type = 'client'
					  AND f.entity_id = u.id
					  AND f.status != 'resolved'
				)
				OR EXISTS (
					SELECT 1
					FROM subscriptions s
					JOIN devices d ON d.id = s.device_id AND d.deleted_at IS NULL
					LEFT JOIN LATERAL (
						SELECT p.status
						FROM payments p
						WHERE p.subscription_id = s.id
						ORDER BY p.created_at DESC
						LIMIT 1
					) p ON TRUE
					WHERE s.org_id = u.org_id
					  AND s.user_id = u.id
					  AND (
						s.status = 'pending'
						OR p.status IN ('pending', 'failed', 'cancelled', 'expired')
						OR (
							(s.status = 'active' OR p.status = 'completed')
							AND d.device_type = 'smartphone'
							AND NULLIF(TRIM(COALESCE(d.imei, '')), '') IS NULL
						)
					  )
				)
				OR EXISTS (
					SELECT 1
					FROM claims c
					WHERE c.org_id = u.org_id
					  AND c.user_id = u.id
					  AND c.status IN ('pending', 'review')
				)
				OR EXISTS (
					SELECT 1
					FROM repair_bookings rb
					WHERE rb.org_id = u.org_id
					  AND rb.user_id = u.id
					  AND rb.status IN ('pending', 'accepted', 'scheduled', 'in_progress')
				)
			) AS requires_attention
		FROM users u
		LEFT JOIN LATERAL (
			SELECT d.id, d.device_type, d.imei, d.status
			FROM devices d
			WHERE d.org_id = u.org_id
			  AND d.user_id = u.id
			  AND d.deleted_at IS NULL
			ORDER BY d.created_at DESC
			LIMIT 1
		) latest_device ON TRUE
		LEFT JOIN LATERAL (
			SELECT s.id, s.status
			FROM subscriptions s
			WHERE latest_device.id IS NOT NULL
			  AND s.device_id = latest_device.id
			ORDER BY s.created_at DESC
			LIMIT 1
		) latest_sub ON TRUE
		LEFT JOIN LATERAL (
			SELECT p.status
			FROM payments p
			WHERE latest_sub.id IS NOT NULL
			  AND p.subscription_id = latest_sub.id
			ORDER BY p.created_at DESC
			LIMIT 1
		) latest_payment ON TRUE
		LEFT JOIN LATERAL (
			SELECT pr.store_name
			FROM partner_clients pc
			JOIN partners pr ON pr.id = pc.partner_id
			WHERE pc.linked_user_id = u.id
			ORDER BY pc.invited_at DESC
			LIMIT 1
		) partner ON TRUE
		WHERE u.org_id = $1
		  AND u.deleted_at IS NULL
		  AND u.role = 'member'
		  AND (
			$2 = ''
			OR lower(u.full_name) LIKE '%' || lower($2) || '%'
			OR lower(u.email) LIKE '%' || lower($2) || '%'
			OR lower(COALESCE(u.phone, '')) LIKE '%' || lower($2) || '%'
		  )
		ORDER BY u.created_at DESC
		LIMIT $3 OFFSET $4
	`, orgID, strings.TrimSpace(search), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		items   []domain.EmployeeClientListItem
		userIDs []uuid.UUID
	)
	for rows.Next() {
		var (
			item          domain.EmployeeClientListItem
			deviceType    *domain.DeviceType
			deviceIMEI    *string
			deviceStatus  *domain.DeviceStatus
			subStatus     *domain.SubscriptionStatus
			paymentStatus *domain.PaymentStatus
		)
		if err := rows.Scan(
			&item.ID,
			&item.FullName,
			&item.Email,
			&item.Phone,
			&item.DeviceCount,
			&item.ActiveSubscriptionCount,
			&item.MissingIMEICount,
			&item.PendingClaimsCount,
			&item.OpenRepairsCount,
			&deviceType,
			&deviceIMEI,
			&deviceStatus,
			&subStatus,
			&paymentStatus,
			&item.PartnerStoreName,
			&item.RequiresAttention,
		); err != nil {
			return nil, err
		}
		item.LatestCoverageStatus = resolveCoverageFromFields(deviceType, deviceIMEI, deviceStatus, subStatus, paymentStatus)
		items = append(items, item)
		userIDs = append(userIDs, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []domain.EmployeeClientListItem{}, nil
	}

	followUps, err := r.fetchFollowUpsByEntityIDs(ctx, orgID, domain.OperationalEntityTypeClient, userIDs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].FollowUp = followUps[items[i].ID]
		if items[i].FollowUp != nil && items[i].FollowUp.Status != domain.FollowUpStatusResolved {
			items[i].RequiresAttention = true
		}
	}

	return items, nil
}

// GetClient returns a detailed employee client view.
func (r *EmployeeRepository) GetClient(ctx context.Context, orgID, userID uuid.UUID) (*domain.EmployeeClientDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	detail := &domain.EmployeeClientDetail{
		Devices:          []domain.EmployeeClientDeviceCoverage{},
		PaymentFollowUps: []domain.EmployeePaymentFollowUpItem{},
		Claims:           []domain.EmployeeClaimDetail{},
		Repairs:          []domain.EmployeeRepairDetail{},
		Notes:            []domain.OperationalNote{},
	}

	err := r.pool.QueryRow(ctx, `
		SELECT
			u.id,
			u.full_name,
			u.email,
			u.phone,
			partner.store_name
		FROM users u
		LEFT JOIN LATERAL (
			SELECT pr.store_name
			FROM partner_clients pc
			JOIN partners pr ON pr.id = pc.partner_id
			WHERE pc.linked_user_id = u.id
			ORDER BY pc.invited_at DESC
			LIMIT 1
		) partner ON TRUE
		WHERE u.org_id = $1
		  AND u.id = $2
		  AND u.deleted_at IS NULL
		  AND u.role = 'member'
	`, orgID, userID).Scan(
		&detail.ID,
		&detail.FullName,
		&detail.Email,
		&detail.Phone,
		&detail.PartnerStoreName,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	devices, err := r.listClientDeviceCoverage(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	detail.Devices = devices

	payments, err := r.listPaymentFollowUpsBase(ctx, orgID, &userID, "", 100, 0)
	if err != nil {
		return nil, err
	}
	detail.PaymentFollowUps = payments

	claims, err := r.listClaimDetailsBase(ctx, orgID, &userID, nil, nil, "", 100, 0)
	if err != nil {
		return nil, err
	}
	detail.Claims = claims

	repairs, err := r.listRepairDetailsBase(ctx, orgID, &userID, nil, nil, "", 100, 0)
	if err != nil {
		return nil, err
	}
	detail.Repairs = repairs

	followUp, err := r.GetFollowUp(ctx, orgID, domain.OperationalEntityTypeClient, userID)
	if err != nil {
		return nil, err
	}
	detail.FollowUp = followUp

	notes, err := r.ListNotes(ctx, orgID, domain.OperationalEntityTypeClient, userID, 50, 0)
	if err != nil {
		return nil, err
	}
	detail.Notes = notes

	return detail, nil
}

// ListPaymentFollowUps returns operational payment/subscription data.
func (r *EmployeeRepository) ListPaymentFollowUps(ctx context.Context, orgID uuid.UUID, search string, limit, offset int) ([]domain.EmployeePaymentFollowUpItem, error) {
	return r.listPaymentFollowUpsBase(ctx, orgID, nil, search, limit, offset)
}

// ListClaims returns operational claims.
func (r *EmployeeRepository) ListClaims(ctx context.Context, orgID uuid.UUID, status *string, search string, limit, offset int) ([]domain.EmployeeClaimDetail, error) {
	return r.listClaimDetailsBase(ctx, orgID, nil, nil, status, search, limit, offset)
}

// GetClaim returns a single operational claim detail.
func (r *EmployeeRepository) GetClaim(ctx context.Context, orgID, claimID uuid.UUID) (*domain.EmployeeClaimDetail, error) {
	claims, err := r.listClaimDetailsBase(ctx, orgID, nil, &claimID, nil, "", 1, 0)
	if err != nil {
		return nil, err
	}
	if len(claims) == 0 {
		return nil, nil
	}
	claim := claims[0]
	notes, err := r.ListNotes(ctx, orgID, domain.OperationalEntityTypeClaim, claimID, 50, 0)
	if err != nil {
		return nil, err
	}
	claim.Notes = notes
	return &claim, nil
}

// ListRepairs returns operational repairs.
func (r *EmployeeRepository) ListRepairs(ctx context.Context, orgID uuid.UUID, status *string, search string, limit, offset int) ([]domain.EmployeeRepairDetail, error) {
	return r.listRepairDetailsBase(ctx, orgID, nil, nil, status, search, limit, offset)
}

// GetRepair returns a single operational repair detail.
func (r *EmployeeRepository) GetRepair(ctx context.Context, orgID, repairID uuid.UUID) (*domain.EmployeeRepairDetail, error) {
	repairs, err := r.listRepairDetailsBase(ctx, orgID, nil, &repairID, nil, "", 1, 0)
	if err != nil {
		return nil, err
	}
	if len(repairs) == 0 {
		return nil, nil
	}
	repair := repairs[0]
	notes, err := r.ListNotes(ctx, orgID, domain.OperationalEntityTypeRepair, repairID, 50, 0)
	if err != nil {
		return nil, err
	}
	repair.Notes = notes
	return &repair, nil
}

// ListTasks returns derived operational tasks.
func (r *EmployeeRepository) ListTasks(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]domain.EmployeeTaskItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	paymentItems, err := r.listPaymentFollowUpsBase(ctx, orgID, nil, "", 200, 0)
	if err != nil {
		return nil, err
	}
	claimItems, err := r.listClaimDetailsBase(ctx, orgID, nil, nil, nil, "", 200, 0)
	if err != nil {
		return nil, err
	}
	repairItems, err := r.listRepairDetailsBase(ctx, orgID, nil, nil, nil, "", 200, 0)
	if err != nil {
		return nil, err
	}
	clientItems, err := r.ListClients(ctx, orgID, "", 200, 0)
	if err != nil {
		return nil, err
	}

	tasks := make([]domain.EmployeeTaskItem, 0, len(paymentItems)+len(claimItems)+len(repairItems)+len(clientItems))

	for _, item := range paymentItems {
		if !item.RequiresAttention {
			continue
		}
		entityID := uuid.Nil
		status := string(item.CoverageStatus)
		if item.Subscription != nil {
			entityID = item.Subscription.ID
			status = string(item.Subscription.Status)
		}
		var followUpStatus *domain.FollowUpStatus
		var nextAction *string
		var lastContactAt *time.Time
		if item.FollowUp != nil {
			followUpStatus = &item.FollowUp.Status
			nextAction = item.FollowUp.NextAction
			lastContactAt = item.FollowUp.LastContactAt
		}
		tasks = append(tasks, domain.EmployeeTaskItem{
			ID:               fmt.Sprintf("subscription:%s", entityID),
			EntityType:       domain.OperationalEntityTypeSubscription,
			EntityID:         entityID,
			Title:            item.ClientName,
			Description:      item.Device.Brand + " " + item.Device.Model,
			Reason:           item.AttentionReason,
			Priority:         paymentPriority(item.AttentionReason),
			ClientName:       item.ClientName,
			ClientEmail:      stringPtr(item.ClientEmail),
			ClientPhone:      item.ClientPhone,
			PartnerStoreName: item.PartnerStoreName,
			Status:           status,
			FollowUpStatus:   followUpStatus,
			NextAction:       nextAction,
			LastContactAt:    lastContactAt,
			UpdatedAt:        maxTime(followUpTime(item.FollowUp), subscriptionTime(item.Subscription)),
		})
	}

	for _, item := range claimItems {
		if item.Claim.Status != domain.ClaimStatusPending && (item.FollowUp == nil || item.FollowUp.Status == domain.FollowUpStatusResolved) {
			continue
		}
		var followUpStatus *domain.FollowUpStatus
		var nextAction *string
		var lastContactAt *time.Time
		reason := "claim_pending_review"
		updatedAt := item.Claim.UpdatedAt
		if item.FollowUp != nil {
			followUpStatus = &item.FollowUp.Status
			nextAction = item.FollowUp.NextAction
			lastContactAt = item.FollowUp.LastContactAt
			updatedAt = maxTime(updatedAt, item.FollowUp.UpdatedAt)
			if item.FollowUp.Reason != nil && strings.TrimSpace(*item.FollowUp.Reason) != "" {
				reason = strings.TrimSpace(*item.FollowUp.Reason)
			}
		}
		tasks = append(tasks, domain.EmployeeTaskItem{
			ID:               fmt.Sprintf("claim:%s", item.Claim.ID),
			EntityType:       domain.OperationalEntityTypeClaim,
			EntityID:         item.Claim.ID,
			Title:            item.ClientName,
			Description:      string(item.Claim.ClaimType),
			Reason:           reason,
			Priority:         "high",
			ClientName:       item.ClientName,
			ClientEmail:      stringPtr(item.ClientEmail),
			ClientPhone:      item.ClientPhone,
			PartnerStoreName: item.PartnerStoreName,
			Status:           string(item.Claim.Status),
			FollowUpStatus:   followUpStatus,
			NextAction:       nextAction,
			LastContactAt:    lastContactAt,
			UpdatedAt:        updatedAt,
		})
	}

	for _, item := range repairItems {
		reason := repairReason(item)
		if reason == "" && (item.FollowUp == nil || item.FollowUp.Status == domain.FollowUpStatusResolved) {
			continue
		}
		if reason == "" {
			reason = "manual_follow_up"
		}
		var followUpStatus *domain.FollowUpStatus
		var nextAction *string
		var lastContactAt *time.Time
		updatedAt := item.Repair.UpdatedAt
		if item.FollowUp != nil {
			followUpStatus = &item.FollowUp.Status
			nextAction = item.FollowUp.NextAction
			lastContactAt = item.FollowUp.LastContactAt
			updatedAt = maxTime(updatedAt, item.FollowUp.UpdatedAt)
			if item.FollowUp.Reason != nil && strings.TrimSpace(*item.FollowUp.Reason) != "" && reason == "manual_follow_up" {
				reason = strings.TrimSpace(*item.FollowUp.Reason)
			}
		}
		tasks = append(tasks, domain.EmployeeTaskItem{
			ID:               fmt.Sprintf("repair:%s", item.Repair.ID),
			EntityType:       domain.OperationalEntityTypeRepair,
			EntityID:         item.Repair.ID,
			Title:            item.Repair.CustomerName,
			Description:      item.Repair.Reference,
			Reason:           reason,
			Priority:         repairPriority(reason),
			ClientName:       item.Repair.CustomerName,
			ClientEmail:      item.ClientEmail,
			ClientPhone:      stringPtr(item.Repair.CustomerPhone),
			PartnerStoreName: item.PartnerStoreName,
			Status:           item.Repair.Status,
			FollowUpStatus:   followUpStatus,
			NextAction:       nextAction,
			LastContactAt:    lastContactAt,
			UpdatedAt:        updatedAt,
		})
	}

	for _, item := range clientItems {
		if item.FollowUp == nil || item.FollowUp.Status == domain.FollowUpStatusResolved {
			continue
		}
		reason := "client_follow_up"
		if item.FollowUp.Reason != nil && strings.TrimSpace(*item.FollowUp.Reason) != "" {
			reason = strings.TrimSpace(*item.FollowUp.Reason)
		}
		followUpStatus := item.FollowUp.Status
		tasks = append(tasks, domain.EmployeeTaskItem{
			ID:               fmt.Sprintf("client:%s", item.ID),
			EntityType:       domain.OperationalEntityTypeClient,
			EntityID:         item.ID,
			Title:            item.FullName,
			Description:      item.Email,
			Reason:           reason,
			Priority:         "medium",
			ClientName:       item.FullName,
			ClientEmail:      stringPtr(item.Email),
			ClientPhone:      item.Phone,
			PartnerStoreName: item.PartnerStoreName,
			Status:           string(item.LatestCoverageStatus),
			FollowUpStatus:   &followUpStatus,
			NextAction:       item.FollowUp.NextAction,
			LastContactAt:    item.FollowUp.LastContactAt,
			UpdatedAt:        item.FollowUp.UpdatedAt,
		})
	}

	sort.Slice(tasks, func(i, j int) bool {
		left := taskPriorityRank(tasks[i].Priority)
		right := taskPriorityRank(tasks[j].Priority)
		if left != right {
			return left < right
		}
		return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
	})

	if offset >= len(tasks) {
		return []domain.EmployeeTaskItem{}, nil
	}

	end := offset + limit
	if end > len(tasks) {
		end = len(tasks)
	}
	return tasks[offset:end], nil
}

// GetFollowUp returns a follow-up record for an entity.
func (r *EmployeeRepository) GetFollowUp(ctx context.Context, orgID uuid.UUID, entityType domain.OperationalEntityType, entityID uuid.UUID) (*domain.OperationalFollowUp, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return scanOperationalFollowUpRow(r.pool.QueryRow(ctx, `
		SELECT `+operationalFollowUpColumns+`
		FROM operational_follow_ups
		WHERE org_id = $1
		  AND entity_type = $2
		  AND entity_id = $3
	`, orgID, entityType, entityID))
}

// UpsertFollowUp creates or updates an operational follow-up.
func (r *EmployeeRepository) UpsertFollowUp(ctx context.Context, followUp *domain.OperationalFollowUp) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO operational_follow_ups (
			org_id, entity_type, entity_id, reason, status, next_action, last_contact_at, created_by, updated_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (org_id, entity_type, entity_id)
		DO UPDATE SET
			reason = EXCLUDED.reason,
			status = EXCLUDED.status,
			next_action = EXCLUDED.next_action,
			last_contact_at = EXCLUDED.last_contact_at,
			updated_by = EXCLUDED.updated_by,
			updated_at = now()
		RETURNING `+operationalFollowUpColumns+`
	`, followUp.OrgID, followUp.EntityType, followUp.EntityID, followUp.Reason, followUp.Status, followUp.NextAction, followUp.LastContactAt, followUp.CreatedBy, followUp.UpdatedBy,
	).Scan(
		&followUp.ID,
		&followUp.OrgID,
		&followUp.EntityType,
		&followUp.EntityID,
		&followUp.Reason,
		&followUp.Status,
		&followUp.NextAction,
		&followUp.LastContactAt,
		&followUp.CreatedBy,
		&followUp.UpdatedBy,
		&followUp.CreatedAt,
		&followUp.UpdatedAt,
	)
}

// ListNotes returns operational notes for an entity.
func (r *EmployeeRepository) ListNotes(ctx context.Context, orgID uuid.UUID, entityType domain.OperationalEntityType, entityID uuid.UUID, limit, offset int) ([]domain.OperationalNote, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			n.id,
			n.org_id,
			n.entity_type,
			n.entity_id,
			n.body,
			n.created_by,
			u.full_name,
			n.created_at
		FROM operational_notes n
		LEFT JOIN users u ON u.id = n.created_by
		WHERE n.org_id = $1
		  AND n.entity_type = $2
		  AND n.entity_id = $3
		ORDER BY n.created_at DESC
		LIMIT $4 OFFSET $5
	`, orgID, entityType, entityID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]domain.OperationalNote, 0)
	for rows.Next() {
		var note domain.OperationalNote
		if err := rows.Scan(
			&note.ID,
			&note.OrgID,
			&note.EntityType,
			&note.EntityID,
			&note.Body,
			&note.CreatedBy,
			&note.CreatedByName,
			&note.CreatedAt,
		); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if notes == nil {
		return []domain.OperationalNote{}, nil
	}
	return notes, nil
}

// CreateNote creates an operational note.
func (r *EmployeeRepository) CreateNote(ctx context.Context, note *domain.OperationalNote) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO operational_notes (org_id, entity_type, entity_id, body, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, note.OrgID, note.EntityType, note.EntityID, note.Body, note.CreatedBy).Scan(&note.ID, &note.CreatedAt)
}

func (r *EmployeeRepository) listClientDeviceCoverage(ctx context.Context, orgID, userID uuid.UUID) ([]domain.EmployeeClientDeviceCoverage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			d.id, d.org_id, d.user_id, d.device_type, d.brand, d.model, d.metadata, d.imei, d.status, d.created_at, d.updated_at, d.deleted_at,
			s.id, s.org_id, s.user_id, s.device_id, s.plan_id, s.status, s.billing_cycle, s.market, s.current_period_start, s.current_period_end, s.cancelled_at, s.created_at, s.updated_at,
			p.id, p.org_id, p.user_id, p.plan_id, p.subscription_id, p.amount_minor, p.market, p.currency, p.provider, p.payment_method, p.status, p.provider_ref, p.payment_url, p.idempotency_key, p.provider_payload, p.paid_at, p.failed_at, p.expires_at, p.created_at, p.updated_at,
			pl.name_fr,
			pl.name_en,
			partner.store_name
		FROM devices d
		LEFT JOIN LATERAL (
			SELECT `+subColumns+`
			FROM subscriptions
			WHERE device_id = d.id
			ORDER BY created_at DESC
			LIMIT 1
		) s ON TRUE
		LEFT JOIN LATERAL (
			SELECT `+paymentColumns+`
			FROM payments
			WHERE subscription_id = s.id
			ORDER BY created_at DESC
			LIMIT 1
		) p ON TRUE
		LEFT JOIN plans pl ON pl.id = s.plan_id
		LEFT JOIN LATERAL (
			SELECT pr.store_name
			FROM partner_clients pc
			JOIN partners pr ON pr.id = pc.partner_id
			WHERE pc.linked_user_id = d.user_id
			ORDER BY pc.invited_at DESC
			LIMIT 1
		) partner ON TRUE
		WHERE d.org_id = $1
		  AND d.user_id = $2
		  AND d.deleted_at IS NULL
		ORDER BY d.created_at DESC
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := make([]domain.EmployeeClientDeviceCoverage, 0)
	for rows.Next() {
		device, sub, payment, planNameFR, planNameEN, partnerStoreName, err := scanEmployeeDeviceCoverageRow(rows)
		if err != nil {
			return nil, err
		}
		if device == nil {
			continue
		}
		devices = append(devices, domain.EmployeeClientDeviceCoverage{
			Device:           *device,
			CoverageStatus:   resolveDashboardCoverageStatus(device, sub, payment),
			Subscription:     sub,
			Payment:          payment,
			PlanNameFR:       planNameFR,
			PlanNameEN:       planNameEN,
			PartnerStoreName: partnerStoreName,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if devices == nil {
		return []domain.EmployeeClientDeviceCoverage{}, nil
	}
	return devices, nil
}

func (r *EmployeeRepository) listPaymentFollowUpsBase(ctx context.Context, orgID uuid.UUID, userID *uuid.UUID, search string, limit, offset int) ([]domain.EmployeePaymentFollowUpItem, error) {
	if err := expireEndedSubscriptions(ctx, r.pool, r.timeout, &orgID, userID, nil, nil); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var args []any
	args = append(args, orgID)

	var query strings.Builder
	query.WriteString(`
		WITH latest_subscriptions AS (
			SELECT DISTINCT ON (s.user_id, s.device_id)
				` + subColumns + `
			FROM subscriptions s
			WHERE s.org_id = $1
			ORDER BY s.user_id, s.device_id, s.created_at DESC
		)
		SELECT
			u.id,
			u.full_name,
			u.email,
			u.phone,
			d.id, d.org_id, d.user_id, d.device_type, d.brand, d.model, d.metadata, d.imei, d.status, d.created_at, d.updated_at, d.deleted_at,
			ls.id, ls.org_id, ls.user_id, ls.device_id, ls.plan_id, ls.status, ls.billing_cycle, ls.current_period_start, ls.current_period_end, ls.cancelled_at, ls.created_at, ls.updated_at,
			p.id, p.org_id, p.user_id, p.plan_id, p.subscription_id, p.amount_minor, p.market, p.currency, p.provider, p.payment_method, p.status, p.provider_ref, p.payment_url, p.idempotency_key, p.provider_payload, p.paid_at, p.failed_at, p.expires_at, p.created_at, p.updated_at,
			pl.name_fr,
			pl.name_en,
			CASE
				WHEN EXISTS (
					SELECT 1
					FROM subscriptions prev
					WHERE prev.device_id = ls.device_id
					  AND prev.org_id = ls.org_id
					  AND prev.created_at < ls.created_at
				) THEN 'renewal'
				ELSE 'first_payment'
			END,
			partner.store_name
		FROM latest_subscriptions ls
		JOIN users u ON u.id = ls.user_id
		JOIN devices d ON d.id = ls.device_id AND d.deleted_at IS NULL
		LEFT JOIN LATERAL (
			SELECT ` + paymentColumns + `
			FROM payments
			WHERE subscription_id = ls.id
			ORDER BY created_at DESC
			LIMIT 1
		) p ON TRUE
		JOIN plans pl ON pl.id = ls.plan_id
		LEFT JOIN LATERAL (
			SELECT pr.store_name
			FROM partner_clients pc
			JOIN partners pr ON pr.id = pc.partner_id
			WHERE pc.linked_user_id = u.id
			ORDER BY pc.invited_at DESC
			LIMIT 1
		) partner ON TRUE
		WHERE u.org_id = $1
		  AND u.deleted_at IS NULL
		  AND u.role = 'member'
	`)

	if userID != nil {
		args = append(args, *userID)
		query.WriteString(fmt.Sprintf(" AND u.id = $%d", len(args)))
	}

	args = append(args, strings.TrimSpace(search))
	searchIdx := len(args)
	query.WriteString(fmt.Sprintf(`
		  AND (
			$%d = ''
			OR lower(u.full_name) LIKE '%%' || lower($%d) || '%%'
			OR lower(u.email) LIKE '%%' || lower($%d) || '%%'
			OR lower(COALESCE(u.phone, '')) LIKE '%%' || lower($%d) || '%%'
			OR lower(d.brand) LIKE '%%' || lower($%d) || '%%'
			OR lower(d.model) LIKE '%%' || lower($%d) || '%%'
		  )
	`, searchIdx, searchIdx, searchIdx, searchIdx, searchIdx, searchIdx))

	args = append(args, limit, offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	query.WriteString(fmt.Sprintf(`
		ORDER BY COALESCE(p.created_at, ls.created_at) DESC, ls.created_at DESC
		LIMIT $%d OFFSET $%d
	`, limitIdx, offsetIdx))

	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.EmployeePaymentFollowUpItem, 0)
	subscriptionIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		item, err := scanEmployeePaymentFollowUpRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
		if item.Subscription != nil {
			subscriptionIDs = append(subscriptionIDs, item.Subscription.ID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []domain.EmployeePaymentFollowUpItem{}, nil
	}

	followUps, err := r.fetchFollowUpsByEntityIDs(ctx, orgID, domain.OperationalEntityTypeSubscription, subscriptionIDs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Subscription != nil {
			items[i].FollowUp = followUps[items[i].Subscription.ID]
		}
		hydratePaymentAttention(&items[i])
	}

	return items, nil
}

func (r *EmployeeRepository) listClaimDetailsBase(ctx context.Context, orgID uuid.UUID, userID *uuid.UUID, claimID *uuid.UUID, status *string, search string, limit, offset int) ([]domain.EmployeeClaimDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var args []any
	args = append(args, orgID)

	var query strings.Builder
	query.WriteString(`
		SELECT
			c.id, c.org_id, c.user_id, c.device_id, c.subscription_id, c.claim_type, c.description, c.status, c.amount_minor, c.filed_at, c.reviewed_at, c.settled_at, c.created_at, c.updated_at,
			u.full_name,
			u.email,
			u.phone,
			d.brand,
			d.model,
			d.device_type,
			d.status,
			d.imei,
			s.status,
			p.status,
			pl.name_fr,
			pl.name_en,
			partner.store_name
		FROM claims c
		JOIN users u ON u.id = c.user_id
		JOIN devices d ON d.id = c.device_id AND d.deleted_at IS NULL
		JOIN subscriptions s ON s.id = c.subscription_id
		LEFT JOIN LATERAL (
			SELECT p.status
			FROM payments p
			WHERE p.subscription_id = s.id
			ORDER BY p.created_at DESC
			LIMIT 1
		) p ON TRUE
		LEFT JOIN plans pl ON pl.id = s.plan_id
		LEFT JOIN LATERAL (
			SELECT pr.store_name
			FROM partner_clients pc
			JOIN partners pr ON pr.id = pc.partner_id
			WHERE pc.linked_user_id = u.id
			ORDER BY pc.invited_at DESC
			LIMIT 1
		) partner ON TRUE
		WHERE c.org_id = $1
	`)

	if userID != nil {
		args = append(args, *userID)
		query.WriteString(fmt.Sprintf(" AND c.user_id = $%d", len(args)))
	}
	if claimID != nil {
		args = append(args, *claimID)
		query.WriteString(fmt.Sprintf(" AND c.id = $%d", len(args)))
	}
	if status != nil {
		args = append(args, *status)
		query.WriteString(fmt.Sprintf(" AND c.status = $%d", len(args)))
	}

	args = append(args, strings.TrimSpace(search))
	searchIdx := len(args)
	query.WriteString(fmt.Sprintf(`
		  AND (
			$%d = ''
			OR lower(u.full_name) LIKE '%%' || lower($%d) || '%%'
			OR lower(u.email) LIKE '%%' || lower($%d) || '%%'
			OR lower(COALESCE(u.phone, '')) LIKE '%%' || lower($%d) || '%%'
			OR lower(d.brand) LIKE '%%' || lower($%d) || '%%'
			OR lower(d.model) LIKE '%%' || lower($%d) || '%%'
			OR lower(c.claim_type) LIKE '%%' || lower($%d) || '%%'
		  )
	`, searchIdx, searchIdx, searchIdx, searchIdx, searchIdx, searchIdx, searchIdx))

	args = append(args, limit, offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	query.WriteString(fmt.Sprintf(`
		ORDER BY c.filed_at DESC
		LIMIT $%d OFFSET $%d
	`, limitIdx, offsetIdx))

	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.EmployeeClaimDetail, 0)
	claimIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		item, err := scanEmployeeClaimDetailRow(rows)
		if err != nil {
			return nil, err
		}
		item.Notes = []domain.OperationalNote{}
		items = append(items, *item)
		claimIDs = append(claimIDs, item.Claim.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []domain.EmployeeClaimDetail{}, nil
	}

	followUps, err := r.fetchFollowUpsByEntityIDs(ctx, orgID, domain.OperationalEntityTypeClaim, claimIDs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].FollowUp = followUps[items[i].Claim.ID]
	}

	return items, nil
}

func (r *EmployeeRepository) listRepairDetailsBase(ctx context.Context, orgID uuid.UUID, userID *uuid.UUID, repairID *uuid.UUID, status *string, search string, limit, offset int) ([]domain.EmployeeRepairDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var args []any
	args = append(args, orgID)

	var query strings.Builder
	query.WriteString(`
		SELECT
			rb.id, rb.org_id, rb.user_id, rb.reference, rb.device_brand, rb.device_model, rb.repair_type, rb.service_mode, rb.center_id,
			rb.preferred_date, rb.preferred_time, rb.scheduled_date, rb.scheduled_time, rb.customer_name, rb.customer_phone, rb.customer_phone_normalized,
			rb.status, rb.repair_amount_minor, rb.request_source, rb.created_at, rb.updated_at,
			u.id,
			u.email,
			partner.store_name
		FROM repair_bookings rb
		LEFT JOIN users u ON u.id = rb.user_id
		LEFT JOIN LATERAL (
			SELECT pr.store_name
			FROM partner_clients pc
			JOIN partners pr ON pr.id = pc.partner_id
			WHERE u.id IS NOT NULL
			  AND pc.linked_user_id = u.id
			ORDER BY pc.invited_at DESC
			LIMIT 1
		) partner ON TRUE
		WHERE rb.org_id = $1
	`)

	if userID != nil {
		args = append(args, *userID)
		query.WriteString(fmt.Sprintf(" AND rb.user_id = $%d", len(args)))
	}
	if repairID != nil {
		args = append(args, *repairID)
		query.WriteString(fmt.Sprintf(" AND rb.id = $%d", len(args)))
	}
	if status != nil {
		args = append(args, *status)
		query.WriteString(fmt.Sprintf(" AND rb.status = $%d", len(args)))
	}

	args = append(args, strings.TrimSpace(search))
	searchIdx := len(args)
	query.WriteString(fmt.Sprintf(`
		  AND (
			$%d = ''
			OR lower(rb.reference) LIKE '%%' || lower($%d) || '%%'
			OR lower(rb.customer_name) LIKE '%%' || lower($%d) || '%%'
			OR lower(rb.customer_phone) LIKE '%%' || lower($%d) || '%%'
			OR lower(rb.device_brand) LIKE '%%' || lower($%d) || '%%'
			OR lower(rb.device_model) LIKE '%%' || lower($%d) || '%%'
			OR lower(COALESCE(u.email, '')) LIKE '%%' || lower($%d) || '%%'
		  )
	`, searchIdx, searchIdx, searchIdx, searchIdx, searchIdx, searchIdx, searchIdx))

	args = append(args, limit, offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	query.WriteString(fmt.Sprintf(`
		ORDER BY rb.created_at DESC
		LIMIT $%d OFFSET $%d
	`, limitIdx, offsetIdx))

	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.EmployeeRepairDetail, 0)
	repairIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		item, err := scanEmployeeRepairDetailRow(rows)
		if err != nil {
			return nil, err
		}
		item.Notes = []domain.OperationalNote{}
		items = append(items, *item)
		repairIDs = append(repairIDs, item.Repair.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []domain.EmployeeRepairDetail{}, nil
	}

	followUps, err := r.fetchFollowUpsByEntityIDs(ctx, orgID, domain.OperationalEntityTypeRepair, repairIDs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].FollowUp = followUps[items[i].Repair.ID]
	}

	return items, nil
}

func (r *EmployeeRepository) fetchFollowUpsByEntityIDs(ctx context.Context, orgID uuid.UUID, entityType domain.OperationalEntityType, entityIDs []uuid.UUID) (map[uuid.UUID]*domain.OperationalFollowUp, error) {
	result := make(map[uuid.UUID]*domain.OperationalFollowUp, len(entityIDs))
	if len(entityIDs) == 0 {
		return result, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+operationalFollowUpColumns+`
		FROM operational_follow_ups
		WHERE org_id = $1
		  AND entity_type = $2
		  AND entity_id = ANY($3)
	`, orgID, entityType, entityIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		followUp, err := scanOperationalFollowUpRow(rows)
		if err != nil {
			return nil, err
		}
		if followUp != nil {
			result[followUp.EntityID] = followUp
		}
	}
	return result, rows.Err()
}

func scanOperationalFollowUpRow(scanner interface{ Scan(dest ...any) error }) (*domain.OperationalFollowUp, error) {
	var followUp domain.OperationalFollowUp
	err := scanner.Scan(
		&followUp.ID,
		&followUp.OrgID,
		&followUp.EntityType,
		&followUp.EntityID,
		&followUp.Reason,
		&followUp.Status,
		&followUp.NextAction,
		&followUp.LastContactAt,
		&followUp.CreatedBy,
		&followUp.UpdatedBy,
		&followUp.CreatedAt,
		&followUp.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &followUp, nil
}

func scanEmployeeDeviceCoverageRow(scanner interface{ Scan(dest ...any) error }) (*domain.Device, *domain.Subscription, *domain.Payment, *string, *string, *string, error) {
	var (
		device           domain.Device
		deviceIMEI       *string
		deviceMeta       []byte
		sub              domain.Subscription
		subID            *uuid.UUID
		subOrgID         *uuid.UUID
		subUserID        *uuid.UUID
		subDeviceID      *uuid.UUID
		subPlanID        *uuid.UUID
		subStatus        *domain.SubscriptionStatus
		subBillingCycle  *string
		subMarket        *string
		subCreatedAt     *time.Time
		subUpdatedAt     *time.Time
		payment          domain.Payment
		paymentID        *uuid.UUID
		paymentOrgID     *uuid.UUID
		paymentUserID    *uuid.UUID
		paymentPlanID    *uuid.UUID
		paymentSubID     *uuid.UUID
		paymentAmount    *int
		paymentMarket    *string
		paymentCurrency  *string
		paymentProvider  *string
		paymentMethod    *string
		paymentStatus    *domain.PaymentStatus
		providerRef      *string
		paymentURL       *string
		idempotency      *string
		payload          []byte
		paymentCreatedAt *time.Time
		paymentUpdatedAt *time.Time
		planNameFR       *string
		planNameEN       *string
		partnerStoreName *string
	)

	err := scanner.Scan(
		&device.ID,
		&device.OrgID,
		&device.UserID,
		&device.DeviceType,
		&device.Brand,
		&device.Model,
		&deviceMeta,
		&deviceIMEI,
		&device.Status,
		&device.CreatedAt,
		&device.UpdatedAt,
		&device.DeletedAt,
		&subID,
		&subOrgID,
		&subUserID,
		&subDeviceID,
		&subPlanID,
		&subStatus,
		&subBillingCycle,
		&subMarket,
		&sub.CurrentPeriodStart,
		&sub.CurrentPeriodEnd,
		&sub.CancelledAt,
		&subCreatedAt,
		&subUpdatedAt,
		&paymentID,
		&paymentOrgID,
		&paymentUserID,
		&paymentPlanID,
		&paymentSubID,
		&paymentAmount,
		&paymentMarket,
		&paymentCurrency,
		&paymentProvider,
		&paymentMethod,
		&paymentStatus,
		&providerRef,
		&paymentURL,
		&idempotency,
		&payload,
		&payment.PaidAt,
		&payment.FailedAt,
		&payment.ExpiresAt,
		&paymentCreatedAt,
		&paymentUpdatedAt,
		&planNameFR,
		&planNameEN,
		&partnerStoreName,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	device.IMEI = scanIMEI(deviceIMEI)
	device.Metadata = scanDeviceMetadata(deviceMeta)

	var subPtr *domain.Subscription
	if subID != nil {
		sub.ID = *subID
		sub.OrgID = *subOrgID
		sub.UserID = *subUserID
		sub.DeviceID = *subDeviceID
		sub.PlanID = *subPlanID
		sub.Status = *subStatus
		sub.BillingCycle = *subBillingCycle
		sub.Market = domain.MarketCode(*subMarket)
		sub.CreatedAt = *subCreatedAt
		sub.UpdatedAt = *subUpdatedAt
		subPtr = &sub
	}

	var paymentPtr *domain.Payment
	if paymentID != nil {
		payment.ID = *paymentID
		payment.OrgID = *paymentOrgID
		payment.UserID = *paymentUserID
		payment.PlanID = *paymentPlanID
		payment.SubscriptionID = *paymentSubID
		payment.AmountMinor = *paymentAmount
		payment.Market = domain.MarketCode(*paymentMarket)
		payment.Currency = *paymentCurrency
		payment.Provider = *paymentProvider
		payment.PaymentMethod = paymentMethod
		payment.Status = *paymentStatus
		payment.ProviderRef = providerRef
		payment.PaymentURL = paymentURL
		payment.IdempotencyKey = idempotency
		payment.ProviderPayload = payload
		payment.CreatedAt = *paymentCreatedAt
		payment.UpdatedAt = *paymentUpdatedAt
		paymentPtr = &payment
	}

	return &device, subPtr, paymentPtr, planNameFR, planNameEN, partnerStoreName, nil
}

func scanEmployeePaymentFollowUpRow(rows pgx.Rows) (*domain.EmployeePaymentFollowUpItem, error) {
	var (
		item             domain.EmployeePaymentFollowUpItem
		deviceMeta       []byte
		deviceIMEI       *string
		sub              domain.Subscription
		payment          domain.Payment
		paymentID        *uuid.UUID
		paymentOrgID     *uuid.UUID
		paymentUserID    *uuid.UUID
		paymentPlanID    *uuid.UUID
		paymentSubID     *uuid.UUID
		paymentAmount    *int
		paymentMarket    *string
		paymentCurrency  *string
		paymentProvider  *string
		paymentMethod    *string
		paymentStatus    *domain.PaymentStatus
		providerRef      *string
		paymentURL       *string
		idempotencyKey   *string
		providerPayload  []byte
		paymentCreatedAt *time.Time
		paymentUpdatedAt *time.Time
		planNameFR       *string
		planNameEN       *string
		paymentContext   string
	)

	item.Subscription = &sub
	item.Device = domain.Device{}
	err := rows.Scan(
		&item.UserID,
		&item.ClientName,
		&item.ClientEmail,
		&item.ClientPhone,
		&item.Device.ID,
		&item.Device.OrgID,
		&item.Device.UserID,
		&item.Device.DeviceType,
		&item.Device.Brand,
		&item.Device.Model,
		&deviceMeta,
		&deviceIMEI,
		&item.Device.Status,
		&item.Device.CreatedAt,
		&item.Device.UpdatedAt,
		&item.Device.DeletedAt,
		&sub.ID,
		&sub.OrgID,
		&sub.UserID,
		&sub.DeviceID,
		&sub.PlanID,
		&sub.Status,
		&sub.BillingCycle,
		&sub.CurrentPeriodStart,
		&sub.CurrentPeriodEnd,
		&sub.CancelledAt,
		&sub.CreatedAt,
		&sub.UpdatedAt,
		&paymentID,
		&paymentOrgID,
		&paymentUserID,
		&paymentPlanID,
		&paymentSubID,
		&paymentAmount,
		&paymentMarket,
		&paymentCurrency,
		&paymentProvider,
		&paymentMethod,
		&paymentStatus,
		&providerRef,
		&paymentURL,
		&idempotencyKey,
		&providerPayload,
		&payment.PaidAt,
		&payment.FailedAt,
		&payment.ExpiresAt,
		&paymentCreatedAt,
		&paymentUpdatedAt,
		&planNameFR,
		&planNameEN,
		&paymentContext,
		&item.PartnerStoreName,
	)
	if err != nil {
		return nil, err
	}

	item.Device.IMEI = scanIMEI(deviceIMEI)
	item.Device.Metadata = scanDeviceMetadata(deviceMeta)
	item.PlanNameFR = planNameFR
	item.PlanNameEN = planNameEN
	item.PaymentContext = domain.PaymentFollowUpContext(paymentContext)
	item.CoverageStatus = resolveDashboardCoverageStatus(&item.Device, &sub, nil)

	if paymentID != nil {
		payment.ID = *paymentID
		payment.OrgID = *paymentOrgID
		payment.UserID = *paymentUserID
		payment.PlanID = *paymentPlanID
		payment.SubscriptionID = *paymentSubID
		payment.AmountMinor = *paymentAmount
		payment.Market = domain.MarketCode(*paymentMarket)
		payment.Currency = *paymentCurrency
		payment.Provider = *paymentProvider
		payment.PaymentMethod = paymentMethod
		payment.Status = *paymentStatus
		payment.ProviderRef = providerRef
		payment.PaymentURL = paymentURL
		payment.IdempotencyKey = idempotencyKey
		payment.ProviderPayload = providerPayload
		payment.CreatedAt = *paymentCreatedAt
		payment.UpdatedAt = *paymentUpdatedAt
		item.Payment = &payment
		item.CoverageStatus = resolveDashboardCoverageStatus(&item.Device, &sub, item.Payment)
	}

	return &item, nil
}

func scanEmployeeClaimDetailRow(rows pgx.Rows) (*domain.EmployeeClaimDetail, error) {
	var (
		item          domain.EmployeeClaimDetail
		deviceStatus  domain.DeviceStatus
		deviceIMEI    *string
		subStatus     domain.SubscriptionStatus
		paymentStatus *domain.PaymentStatus
	)
	err := rows.Scan(
		&item.Claim.ID,
		&item.Claim.OrgID,
		&item.Claim.UserID,
		&item.Claim.DeviceID,
		&item.Claim.SubscriptionID,
		&item.Claim.ClaimType,
		&item.Claim.Description,
		&item.Claim.Status,
		&item.Claim.AmountMinor,
		&item.Claim.FiledAt,
		&item.Claim.ReviewedAt,
		&item.Claim.SettledAt,
		&item.Claim.CreatedAt,
		&item.Claim.UpdatedAt,
		&item.ClientName,
		&item.ClientEmail,
		&item.ClientPhone,
		&item.DeviceBrand,
		&item.DeviceModel,
		&item.DeviceType,
		&deviceStatus,
		&deviceIMEI,
		&subStatus,
		&paymentStatus,
		&item.PlanNameFR,
		&item.PlanNameEN,
		&item.PartnerStoreName,
	)
	if err != nil {
		return nil, err
	}
	item.SubscriptionStatus = subStatus
	item.CoverageStatus = resolveCoverageFromFields(&item.DeviceType, deviceIMEI, &deviceStatus, &subStatus, paymentStatus)
	return &item, nil
}

func scanEmployeeRepairDetailRow(rows pgx.Rows) (*domain.EmployeeRepairDetail, error) {
	var (
		item          domain.EmployeeRepairDetail
		preferredDate time.Time
		scheduledDate *time.Time
	)
	err := rows.Scan(
		&item.Repair.ID,
		&item.Repair.OrgID,
		&item.Repair.UserID,
		&item.Repair.Reference,
		&item.Repair.DeviceBrand,
		&item.Repair.DeviceModel,
		&item.Repair.RepairType,
		&item.Repair.ServiceMode,
		&item.Repair.CenterID,
		&preferredDate,
		&item.Repair.PreferredTime,
		&scheduledDate,
		&item.Repair.ScheduledTime,
		&item.Repair.CustomerName,
		&item.Repair.CustomerPhone,
		&item.Repair.CustomerPhoneNormalized,
		&item.Repair.Status,
		&item.Repair.RepairAmountMinor,
		&item.Repair.RequestSource,
		&item.Repair.CreatedAt,
		&item.Repair.UpdatedAt,
		&item.ClientID,
		&item.ClientEmail,
		&item.PartnerStoreName,
	)
	if err != nil {
		return nil, err
	}
	item.Repair.PreferredDate = preferredDate.Format("2006-01-02")
	if scheduledDate != nil {
		formatted := scheduledDate.Format("2006-01-02")
		item.Repair.ScheduledDate = &formatted
	}
	return &item, nil
}

func resolveCoverageFromFields(deviceType *domain.DeviceType, deviceIMEI *string, deviceStatus *domain.DeviceStatus, subStatus *domain.SubscriptionStatus, paymentStatus *domain.PaymentStatus) domain.DashboardCoverageStatus {
	if deviceType == nil || deviceStatus == nil {
		return domain.DashboardCoverageStatusPending
	}

	device := &domain.Device{
		DeviceType: *deviceType,
		IMEI:       scanIMEI(deviceIMEI),
		Status:     *deviceStatus,
	}

	var sub *domain.Subscription
	if subStatus != nil {
		sub = &domain.Subscription{Status: *subStatus}
	}

	var payment *domain.Payment
	if paymentStatus != nil {
		payment = &domain.Payment{Status: *paymentStatus}
	}

	return resolveDashboardCoverageStatus(device, sub, payment)
}

func hydratePaymentAttention(item *domain.EmployeePaymentFollowUpItem) {
	systemReason := paymentAttentionReason(item)
	manualFollowUp := item.FollowUp != nil && item.FollowUp.Status != domain.FollowUpStatusResolved

	item.AttentionReason = systemReason
	item.RequiresAttention = systemReason != ""

	if manualFollowUp {
		item.RequiresAttention = true
		if item.AttentionReason == "" {
			if item.FollowUp.Reason != nil && strings.TrimSpace(*item.FollowUp.Reason) != "" {
				item.AttentionReason = strings.TrimSpace(*item.FollowUp.Reason)
			} else {
				item.AttentionReason = "manual_follow_up"
			}
		}
	}
}

func paymentAttentionReason(item *domain.EmployeePaymentFollowUpItem) string {
	switch item.CoverageStatus {
	case domain.DashboardCoverageStatusAwaitingPayment:
		return "payment_pending"
	case domain.DashboardCoverageStatusFailed:
		return "payment_failed"
	case domain.DashboardCoverageStatusCancelled:
		return "payment_cancelled"
	case domain.DashboardCoverageStatusExpired:
		return "payment_expired"
	case domain.DashboardCoverageStatusPendingActivation:
		if item.Device.RequiresIMEI() && strings.TrimSpace(item.Device.IMEI) == "" {
			return "activation_missing_imei"
		}
		return "activation_pending"
	case domain.DashboardCoverageStatusPending:
		return "subscription_pending"
	default:
		return ""
	}
}

func repairReason(item domain.EmployeeRepairDetail) string {
	switch item.Repair.Status {
	case domain.RepairStatusPending:
		return "repair_pending"
	case domain.RepairStatusAccepted:
		return "repair_accepted"
	case domain.RepairStatusScheduled:
		if item.Repair.ScheduledDate != nil {
			if scheduledAt, err := time.Parse("2006-01-02", *item.Repair.ScheduledDate); err == nil && scheduledAt.Before(time.Now().Truncate(24*time.Hour)) {
				return "repair_overdue"
			}
		}
		return "repair_scheduled"
	case domain.RepairStatusInProgress:
		if item.Repair.ScheduledDate != nil {
			if scheduledAt, err := time.Parse("2006-01-02", *item.Repair.ScheduledDate); err == nil && scheduledAt.Before(time.Now().Truncate(24*time.Hour)) {
				return "repair_overdue"
			}
		}
		return "repair_in_progress"
	default:
		return ""
	}
}

func paymentPriority(reason string) string {
	switch reason {
	case "payment_failed", "payment_cancelled", "payment_expired", "activation_missing_imei":
		return "high"
	case "payment_pending", "activation_pending", "subscription_pending":
		return "medium"
	default:
		return "medium"
	}
}

func repairPriority(reason string) string {
	switch reason {
	case "repair_overdue", "repair_pending":
		return "high"
	case "repair_scheduled", "repair_in_progress", "repair_accepted":
		return "medium"
	default:
		return "medium"
	}
}

func taskPriorityRank(priority string) int {
	switch priority {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func followUpTime(followUp *domain.OperationalFollowUp) time.Time {
	if followUp == nil {
		return time.Time{}
	}
	return followUp.UpdatedAt
}

func subscriptionTime(subscription *domain.Subscription) time.Time {
	if subscription == nil {
		return time.Time{}
	}
	return subscription.UpdatedAt
}

func maxTime(left, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}
