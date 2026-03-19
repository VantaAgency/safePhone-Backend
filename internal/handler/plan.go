package handler

import (
	"net/http"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// PlanHandler handles plan-related HTTP requests.
type PlanHandler struct {
	svc *service.PlanService
}

// NewPlanHandler creates a new plan handler.
func NewPlanHandler(svc *service.PlanService) *PlanHandler {
	return &PlanHandler{svc: svc}
}

// List returns all available plans.
func (h *PlanHandler) List(w http.ResponseWriter, r *http.Request) {
	plans, err := h.svc.List(r.Context())
	if err != nil {
		WriteError(w, r, domain.InternalError(err))
		return
	}
	WriteSuccess(w, r, http.StatusOK, plans)
}
