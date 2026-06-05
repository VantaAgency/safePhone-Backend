package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// captureRepairRepo records the booking passed to Create so the test can
// assert what market the service inherited onto it.
type captureRepairRepo struct {
	saved *domain.RepairBooking
}

func (r *captureRepairRepo) Create(_ context.Context, b *domain.RepairBooking) error {
	r.saved = b
	return nil
}
func (r *captureRepairRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.RepairBooking, error) {
	return nil, nil
}
func (r *captureRepairRepo) GetByReferenceAndPhone(_ context.Context, _, _ string) (*domain.RepairBooking, error) {
	return nil, nil
}
func (r *captureRepairRepo) ListByOrgAndUser(_ context.Context, _, _ uuid.UUID, _, _ int) ([]domain.RepairBooking, error) {
	return nil, nil
}
func (r *captureRepairRepo) ListByOrg(_ context.Context, _ uuid.UUID, _ *string, _ string, _, _ int) ([]domain.RepairBooking, error) {
	return nil, nil
}
func (r *captureRepairRepo) Update(_ context.Context, _ *domain.RepairBooking) error { return nil }

// marketUserRepo returns a single user with a fixed market.
type marketUserRepo struct {
	user *domain.User
}

func (r *marketUserRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.User, error) {
	return r.user, nil
}
func (r *marketUserRepo) Update(_ context.Context, _ *domain.User) error            { return nil }
func (r *marketUserRepo) UpdateRole(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (r *marketUserRepo) GetEmployeeProfile(_ context.Context, _, _ uuid.UUID) (*domain.EmployeeProfile, error) {
	return nil, nil
}

func TestCreateBookingInheritsMarketFromUser(t *testing.T) {
	cases := []struct {
		name string
		user domain.MarketCode
		want domain.MarketCode
	}{
		{"us user → us repair", domain.MarketUS, domain.MarketUS},
		{"sn user → sn repair", domain.MarketSN, domain.MarketSN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userID := uuid.New()
			repairRepo := &captureRepairRepo{}
			userRepo := &marketUserRepo{user: &domain.User{ID: userID, Market: tc.user}}
			svc := NewRepairService(repairRepo, userRepo)

			ac := &auth.AuthContext{OrgID: uuid.New(), UserID: userID}
			_, appErr := svc.CreateBooking(
				context.Background(), ac,
				"Apple", "iPhone 15", "screen", domain.RepairServiceModeHome,
				nil,
				"2026-07-01", "10:00", "Awa Diop", "+221770000000",
			)
			if appErr != nil {
				t.Fatalf("CreateBooking returned error: %v", appErr)
			}
			if repairRepo.saved == nil {
				t.Fatal("expected a booking to be persisted")
			}
			if repairRepo.saved.Market != tc.want {
				t.Fatalf("booking market = %q, want %q", repairRepo.saved.Market, tc.want)
			}
		})
	}
}

// A public (unauthenticated) request has no user; market stays empty so the
// repository default (SN) applies — the service must not panic on nil ac.
func TestCreateBookingPublicRequestLeavesMarketEmpty(t *testing.T) {
	repairRepo := &captureRepairRepo{}
	userRepo := &marketUserRepo{}
	svc := NewRepairService(repairRepo, userRepo)

	_, appErr := svc.CreateBooking(
		context.Background(), nil,
		"Samsung", "Galaxy S24", "battery", domain.RepairServiceModeHome,
		nil,
		"2026-07-01", "10:00", "Moussa Sow", "+221780000000",
	)
	if appErr != nil {
		t.Fatalf("CreateBooking returned error: %v", appErr)
	}
	if repairRepo.saved == nil {
		t.Fatal("expected a booking to be persisted")
	}
	if repairRepo.saved.Market != "" {
		t.Fatalf("public booking market = %q, want empty (repo defaults to SN)", repairRepo.saved.Market)
	}
}
