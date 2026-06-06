package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// AdminVerificationsHandler exposes the admin endpoints powering the
// /admin → Verifications tab.
type AdminVerificationsHandler struct {
	svc *service.VerificationService
}

// NewAdminVerificationsHandler wires the handler.
func NewAdminVerificationsHandler(svc *service.VerificationService) *AdminVerificationsHandler {
	return &AdminVerificationsHandler{svc: svc}
}

// List returns the queue of devices awaiting verification review.
func (h *AdminVerificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	market, appErr := parseMarketQuery(r.URL.Query().Get("market"))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	devices, appErr := h.svc.List(r.Context(), ac, market, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, devices)
}

// Approve marks a device's verification as approved and activates the
// owning subscription.
func (h *AdminVerificationsHandler) Approve(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	deviceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid device ID"))
		return
	}
	if appErr := h.svc.Approve(r.Context(), ac, deviceID); appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, map[string]string{"status": "approved"})
}

// RejectVerificationRequest carries the explanation shown to the user.
type RejectVerificationRequest struct {
	Reason string `json:"reason" validate:"required,min=3,max=2000"`
}

// Reject rejects a device verification with a reason the user can see.
func (h *AdminVerificationsHandler) Reject(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	deviceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid device ID"))
		return
	}
	var req RejectVerificationRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if appErr := h.svc.Reject(r.Context(), ac, deviceID, req.Reason); appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, map[string]string{"status": "rejected"})
}
