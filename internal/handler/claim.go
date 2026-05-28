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

// CreateClaimRequest is the request body for filing a claim.
type CreateClaimRequest struct {
	DeviceID       string `json:"device_id" validate:"required,uuid"`
	SubscriptionID string `json:"subscription_id" validate:"required,uuid"`
	ClaimType      string `json:"claim_type" validate:"required,oneof=screen water theft breakdown"`
	Description    string `json:"description" validate:"max=5000"`
}

// UpdateClaimStatusRequest is the request body for admin claim status updates.
type UpdateClaimStatusRequest struct {
	Status    string `json:"status" validate:"required,oneof=review approved rejected settled"`
	AmountMinor *int `json:"amount_minor" validate:"omitempty,min=0"`
}

// ClaimHandler handles claim-related HTTP requests.
type ClaimHandler struct {
	svc      *service.ClaimService
	validate *validator.Validate
}

// NewClaimHandler creates a new claim handler.
func NewClaimHandler(svc *service.ClaimService) *ClaimHandler {
	return &ClaimHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Create files a new claim.
func (h *ClaimHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateClaimRequest
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
	subID, _ := uuid.Parse(req.SubscriptionID)

	claim, appErr := h.svc.Create(r.Context(), ac, deviceID, subID, domain.ClaimType(req.ClaimType), req.Description)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, claim)
}

// List returns the authenticated user's claims.
func (h *ClaimHandler) List(w http.ResponseWriter, r *http.Request) {
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

	claims, appErr := h.svc.ListByUser(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, claims)
}

// Get returns a single claim by ID.
func (h *ClaimHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid claim ID"))
		return
	}

	claim, appErr := h.svc.Get(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, claim)
}

// AdminList returns all claims for admin users (paginated).
func (h *ClaimHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	status := r.URL.Query().Get("status")
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	claims, appErr := h.svc.ListByOrg(r.Context(), ac, statusPtr, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, claims)
}

// AdminUpdateStatus updates a claim's status (admin only).
func (h *ClaimHandler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid claim ID"))
		return
	}

	var req UpdateClaimStatusRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	claim, appErr := h.svc.UpdateStatus(r.Context(), ac, id, domain.ClaimStatus(req.Status), req.AmountMinor)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, claim)
}
