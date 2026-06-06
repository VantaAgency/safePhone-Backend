package service

import (
	"context"

	"github.com/google/uuid"

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
func (s *AdminService) ListPayments(ctx context.Context, ac *auth.AuthContext, market string, limit, offset int) ([]domain.AdminPayment, *domain.AppError) {
	payments, err := s.repo.ListPayments(ctx, ac.OrgID, market, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return payments, nil
}

// ListEmployees returns employee records with workload context for the admin dashboard.
func (s *AdminService) ListEmployees(
	ctx context.Context,
	ac *auth.AuthContext,
	search string,
	status *domain.EmployeeAccountStatus,
	sort string,
	limit,
	offset int,
) ([]domain.AdminEmployeeListItem, *domain.AppError) {
	items, err := s.repo.ListEmployees(ctx, ac.OrgID, search, status, sort, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

// GetEmployee returns a single employee detail payload.
func (s *AdminService) GetEmployee(ctx context.Context, ac *auth.AuthContext, userID uuid.UUID) (*domain.AdminEmployeeDetail, *domain.AppError) {
	item, err := s.repo.GetEmployee(ctx, ac.OrgID, userID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if item == nil {
		return nil, domain.NotFound("employee")
	}
	return item, nil
}

// UpdateEmployeeStatus updates an employee access status.
func (s *AdminService) UpdateEmployeeStatus(
	ctx context.Context,
	ac *auth.AuthContext,
	userID uuid.UUID,
	status domain.EmployeeAccountStatus,
	suspendedReason *string,
) (*domain.EmployeeProfile, *domain.AppError) {
	switch status {
	case domain.EmployeeAccountStatusActive, domain.EmployeeAccountStatusInactive, domain.EmployeeAccountStatusSuspended:
	default:
		return nil, domain.BadRequest("invalid employee status")
	}

	profile, err := s.repo.UpdateEmployeeStatus(ctx, ac.OrgID, userID, ac.UserID, status, suspendedReason)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if profile == nil {
		return nil, domain.NotFound("employee")
	}

	return profile, nil
}
