package service

import (
	"context"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// AdminService handles admin-level business logic.
type AdminService struct {
	repo domain.AdminRepository
}

// NewAdminService creates a new admin service.
func NewAdminService(repo domain.AdminRepository) *AdminService {
	return &AdminService{repo: repo}
}

// GetStats returns platform statistics scoped to the admin's org.
func (s *AdminService) GetStats(ctx context.Context, ac *auth.AuthContext) (*domain.AdminStats, *domain.AppError) {
	stats, err := s.repo.GetStats(ctx, ac.OrgID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return stats, nil
}

// ListCustomers returns org customers with optional search.
func (s *AdminService) ListCustomers(ctx context.Context, ac *auth.AuthContext, search string, limit, offset int) ([]domain.AdminCustomer, *domain.AppError) {
	customers, err := s.repo.ListCustomers(ctx, ac.OrgID, search, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return customers, nil
}

// ListPayments returns all payments in the org.
func (s *AdminService) ListPayments(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.AdminPayment, *domain.AppError) {
	payments, err := s.repo.ListPayments(ctx, ac.OrgID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return payments, nil
}
