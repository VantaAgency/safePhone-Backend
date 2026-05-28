package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// SubscriptionService handles subscription business logic.
type SubscriptionService struct {
	repo     domain.SubscriptionRepository
	planRepo domain.PlanRepository
	devMode  bool
}

// NewSubscriptionService creates a new subscription service.
func NewSubscriptionService(repo domain.SubscriptionRepository, planRepo domain.PlanRepository, devMode bool) *SubscriptionService {
	return &SubscriptionService{repo: repo, planRepo: planRepo, devMode: devMode}
}

// Create creates a new subscription for a device and plan.
func (s *SubscriptionService) Create(ctx context.Context, ac *auth.AuthContext, deviceID, planID uuid.UUID, billingCycle string) (*domain.Subscription, *domain.AppError) {
	// Verify plan exists
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if appErr := validatePlanAvailability(plan, s.devMode); appErr != nil {
		return nil, appErr
	}

	now := time.Now()
	var periodEnd time.Time
	if billingCycle == "annual" {
		periodEnd = now.AddDate(1, 0, 0)
	} else {
		periodEnd = now.AddDate(0, 1, 0)
	}

	sub := &domain.Subscription{
		OrgID:              ac.OrgID,
		UserID:             ac.UserID,
		DeviceID:           deviceID,
		PlanID:             planID,
		Status:             domain.SubscriptionStatusPending,
		BillingCycle:       billingCycle,
		CurrentPeriodStart: &now,
		CurrentPeriodEnd:   &periodEnd,
	}

	if err := s.repo.Create(ctx, sub); err != nil {
		return nil, domain.InternalError(err)
	}

	return sub, nil
}

// List returns subscriptions for the authenticated user.
func (s *SubscriptionService) List(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.Subscription, *domain.AppError) {
	subs, err := s.repo.ListByOrgAndUser(ctx, ac.OrgID, ac.UserID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return subs, nil
}

// Get returns a single subscription, verifying ownership.
func (s *SubscriptionService) Get(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*domain.Subscription, *domain.AppError) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if sub == nil {
		return nil, domain.NotFound("subscription")
	}
	if appErr := ac.EnsureOwnership(sub.OrgID, sub.UserID, "subscription"); appErr != nil {
		return nil, appErr
	}
	return sub, nil
}

// Cancel cancels a subscription, verifying ownership.
func (s *SubscriptionService) Cancel(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*domain.Subscription, *domain.AppError) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if sub == nil {
		return nil, domain.NotFound("subscription")
	}
	if appErr := ac.EnsureOwnership(sub.OrgID, sub.UserID, "subscription"); appErr != nil {
		return nil, appErr
	}

	if sub.Status == domain.SubscriptionStatusCancelled {
		return nil, domain.BadRequest("subscription is already cancelled")
	}

	now := time.Now()
	sub.Status = domain.SubscriptionStatusCancelled
	sub.CancelledAt = &now

	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, domain.InternalError(err)
	}

	return sub, nil
}
