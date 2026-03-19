// Package service implements business logic for all domain entities.
package service

import (
	"context"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// PlanService handles plan business logic.
type PlanService struct {
	repo    domain.PlanRepository
	devMode bool
}

// NewPlanService creates a new plan service.
func NewPlanService(repo domain.PlanRepository, devMode bool) *PlanService {
	return &PlanService{repo: repo, devMode: devMode}
}

// List returns all available plans.
func (s *PlanService) List(ctx context.Context) ([]domain.Plan, error) {
	plans, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	return filterVisiblePlans(plans, s.devMode), nil
}
