package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

func TestClaimReferralCreatesPartnerClientForNewUser(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	partnerID := uuid.New()
	phone := "+221770000000"

	repo := &referralTestPartnerRepository{
		partner: &domain.Partner{
			ID:           partnerID,
			OrgID:        orgID,
			StoreName:    "Touba Mobile",
			City:         "Dakar",
			ReferralCode: "ABC12345",
			Status:       "active",
		},
	}
	userRepo := &referralTestUserRepository{
		user: &domain.User{
			ID:       userID,
			OrgID:    orgID,
			FullName: "Aminata Diallo",
			Email:    "aminata@example.com",
			Phone:    &phone,
		},
	}

	svc := NewPartnerService(repo, userRepo, &referralTestPaymentRepository{}, "https://safephone.sn")

	details, appErr := svc.ClaimReferral(
		context.Background(),
		&auth.AuthContext{OrgID: orgID, UserID: userID},
		"abc12345",
		"qr",
	)
	if appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}
	if details == nil {
		t.Fatalf("expected referral details")
	}
	if repo.createdClient == nil {
		t.Fatalf("expected a partner client to be created")
	}
	if repo.createdClient.PartnerID != partnerID {
		t.Fatalf("expected partner %s, got %s", partnerID, repo.createdClient.PartnerID)
	}
	if repo.createdClient.LinkedUserID == nil || *repo.createdClient.LinkedUserID != userID {
		t.Fatalf("expected linked user %s, got %#v", userID, repo.createdClient.LinkedUserID)
	}
	if repo.createdClient.AttributionSource != "partner_referral_link" {
		t.Fatalf("expected partner_referral_link, got %q", repo.createdClient.AttributionSource)
	}
	if repo.createdClient.ReferralMedium != "qr" {
		t.Fatalf("expected qr medium, got %q", repo.createdClient.ReferralMedium)
	}
	if repo.createdClient.ReferralCode == nil || *repo.createdClient.ReferralCode != "ABC12345" {
		t.Fatalf("expected referral code ABC12345, got %#v", repo.createdClient.ReferralCode)
	}
	if details.ReferralLink != "https://safephone.sn/p/ABC12345" {
		t.Fatalf("expected referral link, got %q", details.ReferralLink)
	}
}

func TestClaimReferralIsIdempotentForSamePartner(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	partnerID := uuid.New()

	repo := &referralTestPartnerRepository{
		partner: &domain.Partner{
			ID:           partnerID,
			OrgID:        orgID,
			StoreName:    "Touba Mobile",
			City:         "Dakar",
			ReferralCode: "ABC12345",
			Status:       "active",
		},
		clientByUser: &domain.PartnerClient{
			ID:                uuid.New(),
			OrgID:             orgID,
			PartnerID:         partnerID,
			LinkedUserID:      &userID,
			AttributionSource: "partner_referral_link",
			ReferralMedium:    "share",
		},
	}

	svc := NewPartnerService(
		repo,
		&referralTestUserRepository{},
		&referralTestPaymentRepository{},
		"https://safephone.sn",
	)

	details, appErr := svc.ClaimReferral(
		context.Background(),
		&auth.AuthContext{OrgID: orgID, UserID: userID},
		"ABC12345",
		"share",
	)
	if appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}
	if details == nil {
		t.Fatalf("expected referral details")
	}
	if repo.createdClient != nil {
		t.Fatalf("expected no new partner client to be created")
	}
}

func TestClaimReferralRejectsReattributionToAnotherPartner(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()

	repo := &referralTestPartnerRepository{
		partner: &domain.Partner{
			ID:           uuid.New(),
			OrgID:        orgID,
			StoreName:    "Touba Mobile",
			City:         "Dakar",
			ReferralCode: "ABC12345",
			Status:       "active",
		},
		clientByUser: &domain.PartnerClient{
			ID:           uuid.New(),
			OrgID:        orgID,
			PartnerID:    uuid.New(),
			LinkedUserID: &userID,
		},
	}

	svc := NewPartnerService(
		repo,
		&referralTestUserRepository{},
		&referralTestPaymentRepository{},
		"https://safephone.sn",
	)

	_, appErr := svc.ClaimReferral(
		context.Background(),
		&auth.AuthContext{OrgID: orgID, UserID: userID},
		"ABC12345",
		"share",
	)
	if appErr == nil || appErr.Code != "CONFLICT" {
		t.Fatalf("expected conflict app error, got %#v", appErr)
	}
}

func TestTrackReferralVisitCreatesVisitAndNormalizesSource(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	partnerID := uuid.New()
	repo := &referralTestPartnerRepository{
		partner: &domain.Partner{
			ID:           partnerID,
			OrgID:        orgID,
			StoreName:    "Touba Mobile",
			City:         "Dakar",
			ReferralCode: "ABC12345",
			Status:       "active",
		},
	}

	svc := NewPartnerService(
		repo,
		&referralTestUserRepository{},
		&referralTestPaymentRepository{},
		"https://safephone.sn",
	)

	result, appErr := svc.TrackReferralVisit(context.Background(), "abc12345", "", "share")
	if appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}
	if result == nil {
		t.Fatalf("expected visit result")
	}
	if result.VisitorToken == "" {
		t.Fatalf("expected generated visitor token")
	}
	if len(repo.visits) != 1 {
		t.Fatalf("expected 1 visit, got %d", len(repo.visits))
	}
	if repo.visits[0].SourceMedium != "share" {
		t.Fatalf("expected share medium, got %q", repo.visits[0].SourceMedium)
	}
	if result.Referral == nil || result.Referral.ReferralLink != "https://safephone.sn/p/ABC12345" {
		t.Fatalf("expected referral link, got %#v", result.Referral)
	}
}

type referralTestPartnerRepository struct {
	partner       *domain.Partner
	clientByUser  *domain.PartnerClient
	createdClient *domain.PartnerClient
	visits        []*domain.PartnerReferralVisit
}

func (r *referralTestPartnerRepository) Create(_ context.Context, _ *domain.Partner) error {
	return nil
}

func (r *referralTestPartnerRepository) GetByID(_ context.Context, _ uuid.UUID) (*domain.Partner, error) {
	return r.partner, nil
}

func (r *referralTestPartnerRepository) GetByUser(_ context.Context, _, _ uuid.UUID) (*domain.Partner, error) {
	return r.partner, nil
}

func (r *referralTestPartnerRepository) GetByReferralCode(_ context.Context, code string) (*domain.Partner, error) {
	if r.partner == nil || r.partner.ReferralCode != code {
		return nil, nil
	}
	return r.partner, nil
}

func (r *referralTestPartnerRepository) GetProfile(_ context.Context, _, _ uuid.UUID) (*domain.PartnerProfile, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) CreateClient(_ context.Context, client *domain.PartnerClient) error {
	clone := *client
	if clone.ID == uuid.Nil {
		clone.ID = uuid.New()
	}
	if clone.InvitedAt.IsZero() {
		clone.InvitedAt = time.Now()
	}
	r.createdClient = &clone
	if clone.LinkedUserID != nil {
		r.clientByUser = &clone
	}
	return nil
}

func (r *referralTestPartnerRepository) GetClientByID(_ context.Context, _ uuid.UUID) (*domain.PartnerClient, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) GetClientByLinkedUser(_ context.Context, _, _ uuid.UUID) (*domain.PartnerClient, error) {
	return r.clientByUser, nil
}

func (r *referralTestPartnerRepository) GetClientByInvitationToken(_ context.Context, _ string) (*domain.PartnerClient, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) GetInvitationDetailsByToken(_ context.Context, _ string) (*domain.PartnerInvitationDetails, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) GetReferralDetailsByCode(_ context.Context, code string) (*domain.PartnerReferralDetails, error) {
	if r.partner == nil || r.partner.ReferralCode != code {
		return nil, nil
	}
	return &domain.PartnerReferralDetails{
		PartnerID:        r.partner.ID,
		PartnerStoreName: r.partner.StoreName,
		PartnerCity:      r.partner.City,
		ReferralCode:     r.partner.ReferralCode,
		Status:           r.partner.Status,
	}, nil
}

func (r *referralTestPartnerRepository) ListClients(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.PartnerClient, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) ClaimClientInvitation(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (r *referralTestPartnerRepository) RefreshClientInvitation(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
	return nil
}

func (r *referralTestPartnerRepository) UpdateClientStatus(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID) error {
	return nil
}

func (r *referralTestPartnerRepository) UpdateClientStatusByLinkedUser(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID) error {
	return nil
}

func (r *referralTestPartnerRepository) CreateReferralVisit(_ context.Context, visit *domain.PartnerReferralVisit) error {
	clone := *visit
	r.visits = append(r.visits, &clone)
	return nil
}

func (r *referralTestPartnerRepository) GetReferralMetrics(_ context.Context, _ uuid.UUID) (*domain.PartnerReferralMetrics, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) ListPlanBreakdown(_ context.Context, _ uuid.UUID, _ int) ([]domain.PartnerPlanBreakdown, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) CreateCommission(_ context.Context, _ *domain.PartnerCommission) error {
	return nil
}

func (r *referralTestPartnerRepository) ListSales(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.PartnerSale, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) ListPayouts(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.PartnerPayout, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) ListAll(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.AdminPartner, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) ListAdminCommissions(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.AdminPartnerCommission, error) {
	return nil, nil
}

func (r *referralTestPartnerRepository) ListAdminReferrals(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.AdminPartnerReferral, error) {
	return nil, nil
}

type referralTestUserRepository struct {
	user *domain.User
}

func (r *referralTestUserRepository) GetByID(_ context.Context, _ uuid.UUID) (*domain.User, error) {
	return r.user, nil
}

func (r *referralTestUserRepository) Update(_ context.Context, _ *domain.User) error {
	return nil
}

func (r *referralTestUserRepository) UpdateRole(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (r *referralTestUserRepository) GetEmployeeProfile(_ context.Context, _, _ uuid.UUID) (*domain.EmployeeProfile, error) {
	return nil, nil
}

type referralTestPaymentRepository struct {
	firstSuccessful *domain.Payment
}

func (r *referralTestPaymentRepository) Create(_ context.Context, _ *domain.Payment) error {
	return nil
}

func (r *referralTestPaymentRepository) GetByID(_ context.Context, _ uuid.UUID) (*domain.Payment, error) {
	return nil, nil
}

func (r *referralTestPaymentRepository) GetByIdempotencyKey(_ context.Context, _ string) (*domain.Payment, error) {
	return nil, nil
}

func (r *referralTestPaymentRepository) GetByProviderRef(_ context.Context, _ string) (*domain.Payment, error) {
	return nil, nil
}

func (r *referralTestPaymentRepository) GetFirstSuccessfulByUser(_ context.Context, _, _ uuid.UUID) (*domain.Payment, error) {
	return r.firstSuccessful, nil
}

func (r *referralTestPaymentRepository) ListBySubscriptionID(_ context.Context, _ uuid.UUID, _ int) ([]domain.Payment, error) {
	return nil, nil
}

func (r *referralTestPaymentRepository) ListByOrgAndUser(_ context.Context, _, _ uuid.UUID, _, _ int) ([]domain.Payment, error) {
	return nil, nil
}

func (r *referralTestPaymentRepository) Update(_ context.Context, _ *domain.Payment) error {
	return nil
}
