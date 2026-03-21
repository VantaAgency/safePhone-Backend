package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

type DashboardRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewDashboardRepository(pool *pgxpool.Pool) *DashboardRepository {
	return &DashboardRepository{pool: pool, timeout: 8 * time.Second}
}

func (r *DashboardRepository) GetMemberSummary(
	ctx context.Context,
	orgID, userID uuid.UUID,
) (*domain.MemberDashboardSummary, error) {
	if err := expireEndedSubscriptions(ctx, r.pool, r.timeout, &orgID, &userID, nil, nil); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	summary := &domain.MemberDashboardSummary{
		PendingActivationDevices: []domain.Device{},
		RecentDevices:            []domain.MemberDashboardDevice{},
		RecentClaims:             []domain.Claim{},
		RecentPayments:           []domain.Payment{},
		ActiveSubscriptions:      []domain.MemberDashboardActiveSubscription{},
	}

	if err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*)::int FROM subscriptions WHERE org_id = $1 AND user_id = $2 AND status = 'active'),
			(SELECT COUNT(*)::int FROM devices WHERE org_id = $1 AND user_id = $2 AND deleted_at IS NULL),
			(SELECT COUNT(*)::int FROM claims WHERE org_id = $1 AND user_id = $2),
			(SELECT COUNT(*)::int FROM payments WHERE org_id = $1 AND user_id = $2)
	`, orgID, userID).Scan(
		&summary.ActiveSubscriptionsCount,
		&summary.DevicesCount,
		&summary.ClaimsCount,
		&summary.PaymentsCount,
	); err != nil {
		return nil, err
	}

	recentDevicesRows, err := r.pool.Query(ctx, `
		SELECT
			d.id, d.org_id, d.user_id, d.device_type, d.brand, d.model, d.metadata, d.imei, d.status, d.created_at, d.updated_at, d.deleted_at,
			s.id, s.org_id, s.user_id, s.device_id, s.plan_id, s.status, s.billing_cycle, s.current_period_start, s.current_period_end, s.cancelled_at, s.created_at, s.updated_at,
			p.id, p.org_id, p.user_id, p.plan_id, p.subscription_id, p.amount_xof, p.currency, p.provider, p.payment_method, p.status, p.provider_ref, p.payment_url, p.idempotency_key, p.provider_payload, p.paid_at, p.failed_at, p.expires_at, p.created_at, p.updated_at
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
		WHERE d.org_id = $1
		  AND d.user_id = $2
		  AND d.deleted_at IS NULL
		ORDER BY d.created_at DESC
		LIMIT 3
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer recentDevicesRows.Close()

	for recentDevicesRows.Next() {
		deviceSummary, err := scanMemberDashboardDevice(recentDevicesRows)
		if err != nil {
			return nil, err
		}
		summary.RecentDevices = append(summary.RecentDevices, *deviceSummary)
	}
	if err := recentDevicesRows.Err(); err != nil {
		return nil, err
	}

	pendingActivationRows, err := r.pool.Query(ctx, `
		SELECT
			d.id, d.org_id, d.user_id, d.device_type, d.brand, d.model, d.metadata, d.imei, d.status, d.created_at, d.updated_at, d.deleted_at,
			s.id, s.org_id, s.user_id, s.device_id, s.plan_id, s.status, s.billing_cycle, s.current_period_start, s.current_period_end, s.cancelled_at, s.created_at, s.updated_at,
			p.id, p.org_id, p.user_id, p.plan_id, p.subscription_id, p.amount_xof, p.currency, p.provider, p.payment_method, p.status, p.provider_ref, p.payment_url, p.idempotency_key, p.provider_payload, p.paid_at, p.failed_at, p.expires_at, p.created_at, p.updated_at
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
		WHERE d.org_id = $1
		  AND d.user_id = $2
		  AND d.deleted_at IS NULL
		ORDER BY d.created_at DESC
		LIMIT 8
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer pendingActivationRows.Close()

	for pendingActivationRows.Next() {
		deviceSummary, err := scanMemberDashboardDevice(pendingActivationRows)
		if err != nil {
			return nil, err
		}
		if deviceSummary.CoverageStatus == domain.DashboardCoverageStatusPendingActivation {
			summary.PendingActivationDevices = append(summary.PendingActivationDevices, deviceSummary.Device)
		}
	}
	if err := pendingActivationRows.Err(); err != nil {
		return nil, err
	}

	activeSubscriptionRows, err := r.pool.Query(ctx, `
		SELECT
			s.id, s.org_id, s.user_id, s.device_id, s.plan_id, s.status, s.billing_cycle, s.current_period_start, s.current_period_end, s.cancelled_at, s.created_at, s.updated_at,
			d.id, d.org_id, d.user_id, d.device_type, d.brand, d.model, d.metadata, d.imei, d.status, d.created_at, d.updated_at, d.deleted_at
		FROM subscriptions s
		JOIN devices d ON d.id = s.device_id AND d.deleted_at IS NULL
		WHERE s.org_id = $1
		  AND s.user_id = $2
		  AND s.status = 'active'
		ORDER BY COALESCE(s.current_period_end, s.created_at) ASC NULLS LAST, s.created_at DESC
		LIMIT 5
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer activeSubscriptionRows.Close()

	for activeSubscriptionRows.Next() {
		activeSubscription, err := scanActiveSubscriptionSummary(activeSubscriptionRows)
		if err != nil {
			return nil, err
		}
		summary.ActiveSubscriptions = append(summary.ActiveSubscriptions, *activeSubscription)
	}
	if err := activeSubscriptionRows.Err(); err != nil {
		return nil, err
	}

	recentClaimsRows, err := r.pool.Query(ctx, `
		SELECT `+claimColumns+`
		FROM claims
		WHERE org_id = $1 AND user_id = $2
		ORDER BY filed_at DESC
		LIMIT 3
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer recentClaimsRows.Close()

	claims, err := scanClaims(recentClaimsRows)
	if err != nil {
		return nil, err
	}
	summary.RecentClaims = claims

	recentPaymentsRows, err := r.pool.Query(ctx, `
		SELECT `+paymentColumns+`
		FROM payments
		WHERE org_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT 5
	`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer recentPaymentsRows.Close()

	for recentPaymentsRows.Next() {
		payment, err := scanPaymentRows(recentPaymentsRows)
		if err != nil {
			return nil, err
		}
		summary.RecentPayments = append(summary.RecentPayments, *payment)
	}
	if err := recentPaymentsRows.Err(); err != nil {
		return nil, err
	}

	return summary, nil
}

func (r *DashboardRepository) GetAdminOverview(
	ctx context.Context,
	adminRepo *AdminRepository,
	orgID uuid.UUID,
) (*domain.AdminDashboardOverview, error) {
	stats, err := adminRepo.GetStats(ctx, orgID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	claimsRows, err := r.pool.Query(ctx, `
		SELECT `+claimColumns+`
		FROM claims
		WHERE org_id = $1
		ORDER BY filed_at DESC
		LIMIT 4
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer claimsRows.Close()

	claims, err := scanClaims(claimsRows)
	if err != nil {
		return nil, err
	}

	repairsRows, err := r.pool.Query(ctx, `
		SELECT `+repairColumns+`
		FROM repair_bookings
		WHERE org_id = $1
		ORDER BY created_at DESC
		LIMIT 4
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer repairsRows.Close()

	repairs, err := scanRepairBookings(repairsRows)
	if err != nil {
		return nil, err
	}

	return &domain.AdminDashboardOverview{
		Stats:         *stats,
		RecentClaims:  claims,
		RecentRepairs: repairs,
	}, nil
}

func scanMemberDashboardDevice(scanner interface{ Scan(dest ...any) error }) (*domain.MemberDashboardDevice, error) {
	device, sub, payment, err := scanDashboardDeviceWithCoverage(scanner)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, nil
	}

	return &domain.MemberDashboardDevice{
		Device:         *device,
		CoverageStatus: resolveDashboardCoverageStatus(device, sub, payment),
		Subscription:   sub,
		Payment:        payment,
	}, nil
}

func scanDashboardDeviceWithCoverage(
	scanner interface{ Scan(dest ...any) error },
) (*domain.Device, *domain.Subscription, *domain.Payment, error) {
	var (
		device             domain.Device
		deviceIMEI         *string
		deviceMeta         []byte
		sub                domain.Subscription
		subID              *uuid.UUID
		subOrgID           *uuid.UUID
		subUserID          *uuid.UUID
		subDeviceID        *uuid.UUID
		subPlanID          *uuid.UUID
		subStatus          *domain.SubscriptionStatus
		subBillingCycle    *string
		subCreatedAt       *time.Time
		subUpdatedAt       *time.Time
		payment            domain.Payment
		paymentID          *uuid.UUID
		paymentOrgID       *uuid.UUID
		paymentUserID      *uuid.UUID
		paymentPlanID      *uuid.UUID
		paymentSubID       *uuid.UUID
		paymentAmount      *int
		paymentCurrency    *string
		paymentProvider    *string
		paymentMethod      *string
		paymentStatus      *domain.PaymentStatus
		providerRef        *string
		paymentURL         *string
		idempotency        *string
		payload            []byte
		paymentCreatedAt   *time.Time
		paymentUpdatedAt   *time.Time
	)

	if err := scanner.Scan(
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
	); err != nil {
		return nil, nil, nil, err
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
		payment.AmountXOF = *paymentAmount
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

	return &device, subPtr, paymentPtr, nil
}

func resolveDashboardCoverageStatus(
	device *domain.Device,
	sub *domain.Subscription,
	payment *domain.Payment,
) domain.DashboardCoverageStatus {
	if sub != nil {
		switch sub.Status {
		case domain.SubscriptionStatusActive:
			if device.Status == domain.DeviceStatusActive {
				return domain.DashboardCoverageStatusActive
			}
			if device.RequiresIMEI() {
				return domain.DashboardCoverageStatusPendingActivation
			}
			return domain.DashboardCoverageStatusActive
		case domain.SubscriptionStatusExpired:
			return domain.DashboardCoverageStatusExpired
		case domain.SubscriptionStatusCancelled:
			return domain.DashboardCoverageStatusCancelled
		}
	}

	if payment != nil {
		switch payment.Status {
		case domain.PaymentStatusCompleted:
			if device.Status == domain.DeviceStatusActive || !device.RequiresIMEI() {
				return domain.DashboardCoverageStatusActive
			}
			return domain.DashboardCoverageStatusPendingActivation
		case domain.PaymentStatusPending:
			return domain.DashboardCoverageStatusAwaitingPayment
		case domain.PaymentStatusFailed:
			return domain.DashboardCoverageStatusFailed
		case domain.PaymentStatusCancelled:
			return domain.DashboardCoverageStatusCancelled
		case domain.PaymentStatusExpired:
			return domain.DashboardCoverageStatusExpired
		case domain.PaymentStatusRefunded:
			return domain.DashboardCoverageStatusRefunded
		}
	}

	if device.Status == domain.DeviceStatusSuspended {
		return domain.DashboardCoverageStatusSuspended
	}
	if device.Status == domain.DeviceStatusActive {
		return domain.DashboardCoverageStatusPendingActivation
	}
	return domain.DashboardCoverageStatusPending
}

func scanActiveSubscriptionSummary(
	scanner interface{ Scan(dest ...any) error },
) (*domain.MemberDashboardActiveSubscription, error) {
	var (
		sub        domain.Subscription
		device     domain.Device
		deviceIMEI *string
		deviceMeta []byte
	)

	if err := scanner.Scan(
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
	); err != nil {
		return nil, err
	}

	device.IMEI = scanIMEI(deviceIMEI)
	device.Metadata = scanDeviceMetadata(deviceMeta)

	return &domain.MemberDashboardActiveSubscription{
		Subscription: sub,
		Device:       &device,
	}, nil
}
