package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// ClaimService handles claim business logic.
type ClaimService struct {
	repo       domain.ClaimRepository
	deviceRepo domain.DeviceRepository
	subRepo    domain.SubscriptionRepository
}

// NewClaimService creates a new claim service.
func NewClaimService(repo domain.ClaimRepository, deviceRepo domain.DeviceRepository, subRepo domain.SubscriptionRepository) *ClaimService {
	return &ClaimService{repo: repo, deviceRepo: deviceRepo, subRepo: subRepo}
}

// Create files a new insurance claim after verifying eligibility.
func (s *ClaimService) Create(ctx context.Context, ac *auth.AuthContext, deviceID, subscriptionID uuid.UUID, claimType domain.ClaimType, description string) (*domain.Claim, *domain.AppError) {
	// 1. Verify device ownership and status
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if device == nil || device.OrgID != ac.OrgID || device.UserID != ac.UserID {
		return nil, domain.NotFound("device")
	}
	if device.Status != domain.DeviceStatusActive {
		return nil, domain.BadRequest("device is not active")
	}

	// 2. Verify subscription ownership and status
	sub, err := s.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if sub == nil || sub.OrgID != ac.OrgID || sub.UserID != ac.UserID {
		return nil, domain.NotFound("subscription")
	}
	if sub.Status != domain.SubscriptionStatusActive {
		return nil, domain.BadRequest("subscription is not active")
	}

	// 3. Verify subscription covers this device
	if sub.DeviceID != deviceID {
		return nil, domain.BadRequest("subscription does not cover this device")
	}

	// 4. Check for duplicate pending/review claims
	exists, err := s.repo.ExistsPendingByDeviceAndType(ctx, ac.OrgID, deviceID, claimType)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if exists {
		return nil, domain.Conflict("a pending claim already exists for this device and type")
	}

	// 5. Create claim
	var desc *string
	if description != "" {
		desc = &description
	}

	claim := &domain.Claim{
		OrgID:          ac.OrgID,
		UserID:         ac.UserID,
		DeviceID:       deviceID,
		SubscriptionID: subscriptionID,
		ClaimType:      claimType,
		Description:    desc,
		Status:         domain.ClaimStatusPending,
	}

	if err := s.repo.Create(ctx, claim); err != nil {
		return nil, domain.InternalError(err)
	}

	return claim, nil
}

// ListByUser returns claims for the authenticated user.
func (s *ClaimService) ListByUser(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.Claim, *domain.AppError) {
	claims, err := s.repo.ListByOrgAndUser(ctx, ac.OrgID, ac.UserID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return claims, nil
}

// Get returns a single claim, verifying ownership.
func (s *ClaimService) Get(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*domain.Claim, *domain.AppError) {
	claim, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if claim == nil || claim.OrgID != ac.OrgID {
		return nil, domain.NotFound("claim")
	}
	return claim, nil
}

// ListByOrg returns all claims in the org (admin use).
func (s *ClaimService) ListByOrg(ctx context.Context, ac *auth.AuthContext, status *string, limit, offset int) ([]domain.Claim, *domain.AppError) {
	claims, err := s.repo.ListByOrg(ctx, ac.OrgID, status, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return claims, nil
}

// UpdateStatus updates a claim's status (admin operation).
func (s *ClaimService) UpdateStatus(ctx context.Context, ac *auth.AuthContext, id uuid.UUID, status domain.ClaimStatus, amountXOF *int) (*domain.Claim, *domain.AppError) {
	claim, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if claim == nil || claim.OrgID != ac.OrgID {
		return nil, domain.NotFound("claim")
	}

	now := time.Now()
	claim.Status = status
	claim.AmountXOF = amountXOF

	switch status {
	case domain.ClaimStatusReview, domain.ClaimStatusApproved, domain.ClaimStatusRejected:
		claim.ReviewedAt = &now
	case domain.ClaimStatusSettled:
		claim.ReviewedAt = &now
		claim.SettledAt = &now
	}

	if err := s.repo.Update(ctx, claim); err != nil {
		return nil, domain.InternalError(err)
	}

	return claim, nil
}
