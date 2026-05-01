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

// CreatePartnerApplicationRequest is the request body for a partner application.
type CreatePartnerApplicationRequest struct {
	StoreName              string `json:"store_name" validate:"required,min=1,max=200"`
	FullName               string `json:"full_name" validate:"required,min=1,max=200"`
	Phone                  string `json:"phone" validate:"required,min=1,max=30"`
	City                   string `json:"city" validate:"required,min=1,max=100"`
	BusinessLocation       string `json:"business_location" validate:"required,min=1,max=200"`
	CommercialReferralCode string `json:"commercial_referral_code" validate:"omitempty,max=16"`
}

// ReviewPartnerApplicationRequest is the request body for admin review.
type ReviewPartnerApplicationRequest struct {
	Decision             string   `json:"decision" validate:"required,oneof=approved rejected"`
	RejectionReason      *string  `json:"rejection_reason,omitempty"`
	CommissionPercentage *float64 `json:"commission_percentage,omitempty"`
}

// PartnerApplicationHandler handles partner application HTTP requests.
type PartnerApplicationHandler struct {
	svc      *service.PartnerApplicationService
	validate *validator.Validate
}

// NewPartnerApplicationHandler creates a new partner application handler.
func NewPartnerApplicationHandler(svc *service.PartnerApplicationService) *PartnerApplicationHandler {
	return &PartnerApplicationHandler{svc: svc, validate: validator.New()}
}

// Submit handles POST /api/v1/partner-applications.
func (h *PartnerApplicationHandler) Submit(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	var req CreatePartnerApplicationRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	app, appErr := h.svc.Submit(r.Context(), ac, req.StoreName, req.FullName, req.Phone, req.City, req.BusinessLocation, req.CommercialReferralCode)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, app)
}

// GetMyApplication handles GET /api/v1/partner-applications/mine.
func (h *PartnerApplicationHandler) GetMyApplication(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	app, appErr := h.svc.GetMyApplication(r.Context(), ac)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, app)
}

// AdminList handles GET /api/v1/admin/partner-applications.
func (h *PartnerApplicationHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	var statusFilter *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusFilter = &s
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	apps, appErr := h.svc.ListApplications(r.Context(), ac, statusFilter, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, apps)
}

// AdminReview handles PUT /api/v1/admin/partner-applications/{id}/review.
func (h *PartnerApplicationHandler) AdminReview(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	idStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(idStr)
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid application ID"))
		return
	}

	var req ReviewPartnerApplicationRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("decision must be 'approved' or 'rejected'", nil))
		return
	}

	app, appErr := h.svc.ReviewApplication(r.Context(), ac, appID, req.Decision, req.RejectionReason, req.CommissionPercentage)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, app)
}
