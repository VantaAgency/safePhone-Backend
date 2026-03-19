package service

import (
	"context"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// ContactService handles contact form submissions.
type ContactService struct {
	repo domain.ContactRepository
}

// NewContactService creates a new contact service.
func NewContactService(repo domain.ContactRepository) *ContactService {
	return &ContactService{repo: repo}
}

// Submit saves a new contact message.
func (s *ContactService) Submit(ctx context.Context, name, email string, subject *string, message string) (*domain.ContactMessage, *domain.AppError) {
	msg := &domain.ContactMessage{
		Name:    name,
		Email:   email,
		Subject: subject,
		Message: message,
	}
	if err := s.repo.Create(ctx, msg); err != nil {
		return nil, domain.InternalError(err)
	}
	return msg, nil
}
