package service

import (
	"context"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/repository"
)

type DashboardService struct {
	repo      *repository.DashboardRepository
	adminRepo *repository.AdminRepository
	partner   domain.PartnerRepository
}

func NewDashboardService(
	repo *repository.DashboardRepository,
	adminRepo *repository.AdminRepository,
	partnerRepo domain.PartnerRepository,
) *DashboardService {
	return &DashboardService{
		repo:      repo,
		adminRepo: adminRepo,
		partner:   partnerRepo,
	}
}

func (s *DashboardService) GetMemberSummary(
	ctx context.Context,
	ac *auth.AuthContext,
) (*domain.MemberDashboardSummary, *domain.AppError) {
	summary, err := s.repo.GetMemberSummary(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return summary, nil
}

func (s *DashboardService) GetAdminOverview(
	ctx context.Context,
	ac *auth.AuthContext,
) (*domain.AdminDashboardOverview, *domain.AppError) {
	overview, err := s.repo.GetAdminOverview(ctx, s.adminRepo, ac.OrgID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return overview, nil
}

func (s *DashboardService) GetPartnerOverview(
	ctx context.Context,
	ac *auth.AuthContext,
) (*domain.PartnerDashboardOverview, *domain.AppError) {
	profile, err := s.partner.GetProfile(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if profile == nil {
		return nil, domain.NotFound("partner profile not found")
	}

	partnerRecord, err := s.partner.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partnerRecord == nil {
		return nil, domain.NotFound("partner profile not found")
	}

	clients, err := s.partner.ListClients(ctx, partnerRecord.ID, 50, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if clients == nil {
		clients = []domain.PartnerClient{}
	}

	return &domain.PartnerDashboardOverview{
		Profile:       profile,
		RecentClients: clients,
	}, nil
}
