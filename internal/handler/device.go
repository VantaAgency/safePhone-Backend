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

// CreateDeviceRequest is the request body for device registration.
type CreateDeviceRequest struct {
	Brand string `json:"brand" validate:"required,min=1,max=100"`
	Model string `json:"model" validate:"required,min=1,max=200"`
	IMEI  string `json:"imei" validate:"omitempty,len=15,numeric"`
}

// UpdateDeviceRequest is the request body for updating a device.
type UpdateDeviceRequest struct {
	Brand string `json:"brand" validate:"required,min=1,max=100"`
	Model string `json:"model" validate:"required,min=1,max=200"`
	IMEI  string `json:"imei" validate:"omitempty,len=15,numeric"`
}

// DeviceHandler handles device-related HTTP requests.
type DeviceHandler struct {
	svc      *service.DeviceService
	validate *validator.Validate
}

// NewDeviceHandler creates a new device handler.
func NewDeviceHandler(svc *service.DeviceService) *DeviceHandler {
	return &DeviceHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Create registers a new device.
func (h *DeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceRequest
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

	device, appErr := h.svc.Create(r.Context(), ac, req.Brand, req.Model, req.IMEI)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, device)
}

// List returns the authenticated user's devices.
func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
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

	devices, appErr := h.svc.List(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, devices)
}

// Get returns a single device by ID.
func (h *DeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid device ID"))
		return
	}

	device, appErr := h.svc.Get(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, device)
}

// Update modifies a device.
func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid device ID"))
		return
	}

	var req UpdateDeviceRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	device, appErr := h.svc.Update(r.Context(), ac, id, req.Brand, req.Model, req.IMEI)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, device)
}

// Delete soft-deletes a device.
func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid device ID"))
		return
	}

	if appErr := h.svc.Delete(r.Context(), ac, id); appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, map[string]string{"status": "deleted"})
}
