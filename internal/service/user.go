package service

import (
	"context"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// UserService handles user profile business logic.
type UserService struct {
	repo domain.UserRepository
}

// NewUserService creates a new user service.
func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// UpdatePhone sets the phone number for the authenticated user.
func (s *UserService) UpdatePhone(ctx context.Context, ac *auth.AuthContext, phone string) (*domain.User, *domain.AppError) {
	user, err := s.repo.GetByID(ctx, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if user == nil {
		return nil, domain.NotFound("user")
	}

	user.Phone = &phone
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, domain.InternalError(err)
	}

	return user, nil
}
