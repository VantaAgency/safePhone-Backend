package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// CreateSubscriptionRequest is the request body for creating a subscription.
type CreateSubscriptionRequest struct {
	DeviceID     string `json:"device_id" validate:"required,uuid"`
	PlanID       string `json:"plan_id" validate:"required,uuid"`
	BillingCycle string `json:"billing_cycle" validate:"required,oneof=monthly annual"`
}

// SubscriptionHandler handles subscription-related HTTP requests.
type SubscriptionHandler struct {
	svc      *service.SubscriptionService
	validate *validator.Validate
}

// NewSubscriptionHandler creates a new subscription handler.
func NewSubscriptionHandler(svc *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Create creates a new subscription.
func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriptionRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	deviceID, _ := uuid.Parse(req.DeviceID)
	planID, _ := uuid.Parse(req.PlanID)

	sub, appErr := h.svc.Create(r.Context(), ac, deviceID, planID, req.BillingCycle)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, sub)
}

// List returns the authenticated user's subscriptions.
func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	subs, appErr := h.svc.List(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, subs)
}

// Get returns a single subscription by ID.
func (h *SubscriptionHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid subscription ID"))
		return
	}

	sub, appErr := h.svc.Get(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, sub)
}

// Cancel cancels a subscription.
func (h *SubscriptionHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid subscription ID"))
		return
	}

	sub, appErr := h.svc.Cancel(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, sub)
}
