package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

func TestCreatePartnerCommissionForFirstSuccessfulPaymentCreatesCommission(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	paymentID := uuid.New()
	partnerID := uuid.New()
	partnerClientID := uuid.New()
	planID := uuid.New()
	paidAt := time.Now()

	payment := &domain.Payment{
		ID:        paymentID,
		OrgID:     orgID,
		UserID:    userID,
		PlanID:    planID,
		AmountXOF: 12000,
		Status:    domain.PaymentStatusCompleted,
		PaidAt:    &paidAt,
		CreatedAt: paidAt.Add(-time.Minute),
		UpdatedAt: paidAt,
	}

	paymentRepo := &stubPaymentRepository{firstSuccessful: payment}
	partnerRepo := &stubPartnerRepository{
		client: &domain.PartnerClient{
			ID:        partnerClientID,
			OrgID:     orgID,
			PartnerID: partnerID,
		},
		partner: &domain.Partner{
			ID:                   partnerID,
			OrgID:                orgID,
			Status:               "active",
			CommissionPercentage: 12.5,
		},
	}

	svc := &PaymentService{
		repo:        paymentRepo,
		partnerRepo: partnerRepo,
	}

	if appErr := svc.createPartnerCommissionForFirstSuccessfulPayment(context.Background(), payment); appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}

	if len(partnerRepo.createdCommissions) != 1 {
		t.Fatalf("expected 1 commission, got %d", len(partnerRepo.createdCommissions))
	}

	created := partnerRepo.createdCommissions[0]
	if created.PartnerID != partnerID {
		t.Fatalf("expected partner %s, got %s", partnerID, created.PartnerID)
	}
	if created.PaymentID == nil || *created.PaymentID != paymentID {
		t.Fatalf("expected payment %s, got %#v", paymentID, created.PaymentID)
	}
	if created.ClientUserID == nil || *created.ClientUserID != userID {
		t.Fatalf("expected user %s, got %#v", userID, created.ClientUserID)
	}
	if created.PartnerClientID == nil || *created.PartnerClientID != partnerClientID {
		t.Fatalf("expected partner client %s, got %#v", partnerClientID, created.PartnerClientID)
	}
	if created.PlanID == nil || *created.PlanID != planID {
		t.Fatalf("expected plan %s, got %#v", planID, created.PlanID)
	}
	if created.BaseAmountXOF != 12000 {
		t.Fatalf("expected base amount 12000, got %d", created.BaseAmountXOF)
	}
	if created.CommissionPercentage != 12.5 {
		t.Fatalf("expected percentage 12.5, got %f", created.CommissionPercentage)
	}
	if created.CommissionAmountXOF != 1500 {
		t.Fatalf("expected commission amount 1500, got %d", created.CommissionAmountXOF)
	}
	if created.Status != "pending" {
		t.Fatalf("expected pending status, got %q", created.Status)
	}
}

func TestCreatePartnerCommissionForFirstSuccessfulPaymentSkipsWhenPaymentIsNotFirst(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	currentPaymentID := uuid.New()
	firstPaymentID := uuid.New()

	currentPayment := &domain.Payment{
		ID:        currentPaymentID,
		OrgID:     orgID,
		UserID:    userID,
		PlanID:    uuid.New(),
		AmountXOF: 9000,
		Status:    domain.PaymentStatusCompleted,
	}

	firstSuccessful := &domain.Payment{
		ID:        firstPaymentID,
		OrgID:     orgID,
		UserID:    userID,
		PlanID:    uuid.New(),
		AmountXOF: 5000,
		Status:    domain.PaymentStatusCompleted,
	}

	partnerRepo := &stubPartnerRepository{
		client: &domain.PartnerClient{ID: uuid.New(), OrgID: orgID, PartnerID: uuid.New()},
		partner: &domain.Partner{
			ID:                   uuid.New(),
			OrgID:                orgID,
			Status:               "active",
			CommissionPercentage: 10,
		},
	}

	svc := &PaymentService{
		repo:        &stubPaymentRepository{firstSuccessful: firstSuccessful},
		partnerRepo: partnerRepo,
	}

	if appErr := svc.createPartnerCommissionForFirstSuccessfulPayment(context.Background(), currentPayment); appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}

	if len(partnerRepo.createdCommissions) != 0 {
		t.Fatalf("expected no commission to be created, got %d", len(partnerRepo.createdCommissions))
	}
}

func TestIsValidCommissionPercentage(t *testing.T) {
	t.Parallel()

	valid := 15.25
	tooHigh := 100.01
	tooPrecise := 10.123
	zero := 0.0

	if !isValidCommissionPercentage(&valid) {
		t.Fatalf("expected %v to be valid", valid)
	}
	if isValidCommissionPercentage(nil) {
		t.Fatalf("expected nil percentage to be invalid")
	}
	if isValidCommissionPercentage(&tooHigh) {
		t.Fatalf("expected %v to be invalid", tooHigh)
	}
	if isValidCommissionPercentage(&tooPrecise) {
		t.Fatalf("expected %v to be invalid", tooPrecise)
	}
	if isValidCommissionPercentage(&zero) {
		t.Fatalf("expected %v to be invalid", zero)
	}
}

func TestCalculateCommissionAmountXOFRoundsToNearestWholeXOF(t *testing.T) {
	t.Parallel()

	if amount := calculateCommissionAmountXOF(999, 12.5); amount != 125 {
		t.Fatalf("expected rounded amount 125, got %d", amount)
	}
}

type stubPaymentRepository struct {
	firstSuccessful *domain.Payment
}

func (s *stubPaymentRepository) Create(_ context.Context, _ *domain.Payment) error {
	return nil
}

func (s *stubPaymentRepository) GetByID(_ context.Context, _ uuid.UUID) (*domain.Payment, error) {
	return nil, nil
}

func (s *stubPaymentRepository) GetByIdempotencyKey(_ context.Context, _ string) (*domain.Payment, error) {
	return nil, nil
}

func (s *stubPaymentRepository) GetByProviderRef(_ context.Context, _ string) (*domain.Payment, error) {
	return nil, nil
}

func (s *stubPaymentRepository) GetFirstSuccessfulByUser(_ context.Context, _, _ uuid.UUID) (*domain.Payment, error) {
	return s.firstSuccessful, nil
}

func (s *stubPaymentRepository) ListBySubscriptionID(_ context.Context, _ uuid.UUID, _ int) ([]domain.Payment, error) {
	return nil, nil
}

func (s *stubPaymentRepository) ListByOrgAndUser(_ context.Context, _, _ uuid.UUID, _, _ int) ([]domain.Payment, error) {
	return nil, nil
}

func (s *stubPaymentRepository) Update(_ context.Context, _ *domain.Payment) error {
	return nil
}

type stubPartnerRepository struct {
	client             *domain.PartnerClient
	partner            *domain.Partner
	createdCommissions []*domain.PartnerCommission
}

func (s *stubPartnerRepository) Create(_ context.Context, _ *domain.Partner) error {
	return nil
}

func (s *stubPartnerRepository) GetByID(_ context.Context, _ uuid.UUID) (*domain.Partner, error) {
	return s.partner, nil
}

func (s *stubPartnerRepository) GetByUser(_ context.Context, _, _ uuid.UUID) (*domain.Partner, error) {
	return s.partner, nil
}

func (s *stubPartnerRepository) GetByReferralCode(_ context.Context, _ string) (*domain.Partner, error) {
	return s.partner, nil
}

func (s *stubPartnerRepository) GetProfile(_ context.Context, _, _ uuid.UUID) (*domain.PartnerProfile, error) {
	return nil, nil
}

func (s *stubPartnerRepository) CreateClient(_ context.Context, _ *domain.PartnerClient) error {
	return nil
}

func (s *stubPartnerRepository) GetClientByID(_ context.Context, _ uuid.UUID) (*domain.PartnerClient, error) {
	return s.client, nil
}

func (s *stubPartnerRepository) GetClientByLinkedUser(_ context.Context, _, _ uuid.UUID) (*domain.PartnerClient, error) {
	return s.client, nil
}

func (s *stubPartnerRepository) GetClientByInvitationToken(_ context.Context, _ string) (*domain.PartnerClient, error) {
	return nil, nil
}

func (s *stubPartnerRepository) GetInvitationDetailsByToken(_ context.Context, _ string) (*domain.PartnerInvitationDetails, error) {
	return nil, nil
}

func (s *stubPartnerRepository) GetReferralDetailsByCode(_ context.Context, _ string) (*domain.PartnerReferralDetails, error) {
	return nil, nil
}

func (s *stubPartnerRepository) ListClients(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.PartnerClient, error) {
	return nil, nil
}

func (s *stubPartnerRepository) ClaimClientInvitation(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func (s *stubPartnerRepository) RefreshClientInvitation(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
	return nil
}

func (s *stubPartnerRepository) UpdateClientStatus(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID) error {
	return nil
}

func (s *stubPartnerRepository) UpdateClientStatusByLinkedUser(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID) error {
	return nil
}

func (s *stubPartnerRepository) CreateReferralVisit(_ context.Context, _ *domain.PartnerReferralVisit) error {
	return nil
}

func (s *stubPartnerRepository) GetReferralMetrics(_ context.Context, _ uuid.UUID) (*domain.PartnerReferralMetrics, error) {
	return nil, nil
}

func (s *stubPartnerRepository) ListPlanBreakdown(_ context.Context, _ uuid.UUID, _ int) ([]domain.PartnerPlanBreakdown, error) {
	return nil, nil
}

func (s *stubPartnerRepository) CreateCommission(_ context.Context, commission *domain.PartnerCommission) error {
	clone := *commission
	s.createdCommissions = append(s.createdCommissions, &clone)
	return nil
}

func (s *stubPartnerRepository) ListSales(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.PartnerSale, error) {
	return nil, nil
}

func (s *stubPartnerRepository) ListPayouts(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.PartnerPayout, error) {
	return nil, nil
}

func (s *stubPartnerRepository) ListAll(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.AdminPartner, error) {
	return nil, nil
}

func (s *stubPartnerRepository) ListAdminCommissions(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.AdminPartnerCommission, error) {
	return nil, nil
}

func (s *stubPartnerRepository) ListAdminReferrals(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.AdminPartnerReferral, error) {
	return nil, nil
}
