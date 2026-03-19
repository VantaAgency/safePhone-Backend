package service

import "github.com/cherif-safephone/safephone-backend/internal/domain"

func isDevelopmentTestPlan(plan *domain.Plan) bool {
	return plan != nil && plan.Slug == domain.DevelopmentTestPlanSlug
}

func filterVisiblePlans(plans []domain.Plan, devMode bool) []domain.Plan {
	if plans == nil {
		return []domain.Plan{}
	}
	if devMode {
		return plans
	}

	filtered := make([]domain.Plan, 0, len(plans))
	for _, plan := range plans {
		if plan.Slug == domain.DevelopmentTestPlanSlug {
			continue
		}
		filtered = append(filtered, plan)
	}
	return filtered
}

func validatePlanAvailability(plan *domain.Plan, devMode bool) *domain.AppError {
	if plan == nil {
		return domain.NotFound("plan")
	}
	if isDevelopmentTestPlan(plan) && !devMode {
		return domain.NotFound("plan")
	}
	return nil
}
