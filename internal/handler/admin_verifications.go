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
	svc         *service.VerificationService
	mediaSecret []byte
}

// NewAdminVerificationsHandler wires the handler. mediaSecret signs the
// verification-media URLs returned to the moderation UI so the browser can
// stream them directly.
func NewAdminVerificationsHandler(svc *service.VerificationService, mediaSecret []byte) *AdminVerificationsHandler {
	return &AdminVerificationsHandler{svc: svc, mediaSecret: mediaSecret}
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

	devices, appErr := h.svc.List(r.Context(), ac, limit, offset)
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

// ListModeration returns covered devices (admin/employee) for fraud review.
func (h *AdminVerificationsHandler) ListModeration(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	devices, appErr := h.svc.ListForModeration(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	// Replace the stored media URLs with short-lived signed paths so the
	// moderation UI can load/stream them without a bearer token.
	for i := range devices {
		for j, p := range devices[i].VerificationPhotos {
			devices[i].VerificationPhotos[j] = SignStoredMediaURL(h.mediaSecret, p)
		}
		if devices[i].VerificationVideo != nil && *devices[i].VerificationVideo != "" {
			signed := SignStoredMediaURL(h.mediaSecret, *devices[i].VerificationVideo)
			devices[i].VerificationVideo = &signed
		}
	}
	WriteSuccess(w, r, http.StatusOK, devices)
}

// Suspend takes a flagged device out of coverage (reversible).
func (h *AdminVerificationsHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	h.moderate(w, r, func(ac *auth.AuthContext, id uuid.UUID) *domain.AppError {
		return h.svc.Suspend(r.Context(), ac, id)
	}, "suspended")
}

// Reactivate restores a suspended device to coverage.
func (h *AdminVerificationsHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	h.moderate(w, r, func(ac *auth.AuthContext, id uuid.UUID) *domain.AppError {
		return h.svc.Reactivate(r.Context(), ac, id)
	}, "active")
}

func (h *AdminVerificationsHandler) moderate(
	w http.ResponseWriter,
	r *http.Request,
	action func(*auth.AuthContext, uuid.UUID) *domain.AppError,
	resultStatus string,
) {
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
	if appErr := action(ac, deviceID); appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, map[string]string{"status": resultStatus})
}
