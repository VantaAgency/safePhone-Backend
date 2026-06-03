package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/repository"
)

// VerificationService backs the admin Verifications tab — the queue of
// devices awaiting review of their photo/video proof before the owning
// subscription activates.
//
// Approve: device.verification_status -> approved, subscription.status
//          -> active, subscription.activated_at -> now().
// Reject:  device.verification_status -> rejected (status stays
//          pending_verification on the sub; the user can re-upload).
type VerificationService struct {
	devices *repository.DeviceRepository
	subs    *repository.SubscriptionRepository
}

// NewVerificationService creates the admin verifications service.
func NewVerificationService(
	devices *repository.DeviceRepository,
	subs *repository.SubscriptionRepository,
) *VerificationService {
	return &VerificationService{devices: devices, subs: subs}
}

// List returns the queue of devices waiting for an admin decision in the
// caller's org, ordered newest-first.
func (s *VerificationService) List(
	ctx context.Context,
	ac *auth.AuthContext,
	limit, offset int,
) ([]domain.Device, *domain.AppError) {
	if !ac.HasRole(auth.RoleAdmin) {
		return nil, domain.Forbidden("admin role required")
	}
	devices, err := s.devices.ListPendingVerifications(ctx, ac.OrgID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return devices, nil
}

// Approve flips the device's verification status and walks the owning
// subscription out of pending_verification. Idempotent on the device side
// (calling twice still ends in approved); the subscription transition is
// guarded against repeat activated_at writes by SetActivatedAt's
// RowsAffected check.
func (s *VerificationService) Approve(
	ctx context.Context,
	ac *auth.AuthContext,
	deviceID uuid.UUID,
) *domain.AppError {
	if !ac.HasRole(auth.RoleAdmin) {
		return domain.Forbidden("admin role required")
	}
	device, err := s.devices.LoadVerification(ctx, deviceID)
	if err != nil {
		return domain.InternalError(err)
	}
	if device == nil || device.OrgID != ac.OrgID {
		return domain.NotFound("device")
	}

	if err := s.devices.SetVerificationDecision(ctx, deviceID, domain.DeviceVerificationStatusApproved, ac.UserID, ""); err != nil {
		return domain.InternalError(err)
	}

	// Bring the device itself to active state — the rest of the system
	// (claims, dashboards) reads device.Status == active as the gate.
	device.Status = domain.DeviceStatusActive
	if err := s.devices.Update(ctx, device); err != nil {
		return domain.InternalError(err)
	}

	// Transition the owning subscription. We look it up via the legacy
	// device_id pointer — subscription_devices doesn't get us there in
	// reverse without a second query and that's a follow-up.
	sub, err := s.subs.GetByDeviceID(ctx, deviceID)
	if err != nil {
		return domain.InternalError(err)
	}
	if sub == nil {
		// No subscription yet (user hasn't paid) — that's fine, the
		// verification stands; nothing else to do.
		return nil
	}
	if sub.Status != domain.SubscriptionStatusPendingVerification {
		// Already past this gate (e.g. legacy sub created before plans
		// v2). Don't reset activated_at.
		return nil
	}
	if err := s.subs.SetActivatedAt(ctx, sub.ID, time.Now()); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.InternalError(err)
	}
	return nil
}

// Reject records the admin's negative decision. The owning subscription
// stays in pending_verification so the user can re-upload media and
// re-trigger the review.
func (s *VerificationService) Reject(
	ctx context.Context,
	ac *auth.AuthContext,
	deviceID uuid.UUID,
	reason string,
) *domain.AppError {
	if !ac.HasRole(auth.RoleAdmin) {
		return domain.Forbidden("admin role required")
	}
	if reason == "" {
		return domain.BadRequest("rejection reason is required so the user knows what to fix")
	}
	device, err := s.devices.LoadVerification(ctx, deviceID)
	if err != nil {
		return domain.InternalError(err)
	}
	if device == nil || device.OrgID != ac.OrgID {
		return domain.NotFound("device")
	}
	if err := s.devices.SetVerificationDecision(ctx, deviceID, domain.DeviceVerificationStatusRejected, ac.UserID, reason); err != nil {
		return domain.InternalError(err)
	}
	return nil
}
