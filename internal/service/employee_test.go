package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

func TestEmployeeUpdateClaimStatusMovesPendingClaimToReview(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	claimID := uuid.New()
	repo := &stubEmployeeClaimRepository{
		claim: &domain.Claim{
			ID:     claimID,
			OrgID:  orgID,
			Status: domain.ClaimStatusPending,
		},
	}
	svc := NewEmployeeService(nil, nil, nil, repo, nil)

	claim, appErr := svc.UpdateClaimStatus(
		context.Background(),
		&auth.AuthContext{OrgID: orgID},
		claimID,
		domain.ClaimStatusReview,
	)
	if appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}
	if claim == nil {
		t.Fatal("expected updated claim, got nil")
	}
	if claim.Status != domain.ClaimStatusReview {
		t.Fatalf("expected review status, got %q", claim.Status)
	}
	if claim.ReviewedAt == nil {
		t.Fatal("expected reviewed_at to be set")
	}
	if repo.updated == nil || repo.updated.Status != domain.ClaimStatusReview {
		t.Fatalf("expected repository update with review status, got %#v", repo.updated)
	}
}

func TestEmployeeUpdateClaimStatusRejectsNonPendingClaims(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	claimID := uuid.New()
	repo := &stubEmployeeClaimRepository{
		claim: &domain.Claim{
			ID:     claimID,
			OrgID:  orgID,
			Status: domain.ClaimStatusApproved,
		},
	}
	svc := NewEmployeeService(nil, nil, nil, repo, nil)

	claim, appErr := svc.UpdateClaimStatus(
		context.Background(),
		&auth.AuthContext{OrgID: orgID},
		claimID,
		domain.ClaimStatusReview,
	)
	if appErr == nil {
		t.Fatalf("expected app error, got claim %#v", claim)
	}
	if appErr.Code != domain.CodeBadRequest {
		t.Fatalf("expected bad request error, got %#v", appErr)
	}
	if repo.updated != nil {
		t.Fatalf("expected no repository update, got %#v", repo.updated)
	}
}

type stubEmployeeClaimRepository struct {
	claim   *domain.Claim
	updated *domain.Claim
}

func (s *stubEmployeeClaimRepository) Create(context.Context, *domain.Claim) error {
	panic("unexpected call to Create")
}

func (s *stubEmployeeClaimRepository) GetByID(context.Context, uuid.UUID) (*domain.Claim, error) {
	return s.claim, nil
}

func (s *stubEmployeeClaimRepository) ListByOrgAndUser(context.Context, uuid.UUID, uuid.UUID, int, int) ([]domain.Claim, error) {
	panic("unexpected call to ListByOrgAndUser")
}

func (s *stubEmployeeClaimRepository) ListByOrg(context.Context, uuid.UUID, *string, int, int) ([]domain.Claim, error) {
	panic("unexpected call to ListByOrg")
}

func (s *stubEmployeeClaimRepository) Update(_ context.Context, claim *domain.Claim) error {
	copied := *claim
	s.updated = &copied
	s.claim = &copied
	return nil
}

func (s *stubEmployeeClaimRepository) ExistsPendingByDeviceAndType(context.Context, uuid.UUID, uuid.UUID, domain.ClaimType) (bool, error) {
	panic("unexpected call to ExistsPendingByDeviceAndType")
}
