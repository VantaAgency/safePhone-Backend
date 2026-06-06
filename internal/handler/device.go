package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

type DeviceMetadataPayload struct {
	SerialNumber     string `json:"serial_number"`
	ScreenSize       string `json:"screen_size"`
	ComputerCategory string `json:"computer_category"`
	ProductSubtype   string `json:"product_subtype"`
}

func (p DeviceMetadataPayload) ToDomain() domain.DeviceMetadata {
	return domain.DeviceMetadata{
		SerialNumber:     strings.TrimSpace(p.SerialNumber),
		ScreenSize:       strings.TrimSpace(p.ScreenSize),
		ComputerCategory: strings.TrimSpace(p.ComputerCategory),
		ProductSubtype:   strings.TrimSpace(p.ProductSubtype),
	}
}

// CreateDeviceRequest is the request body for device registration.
type CreateDeviceRequest struct {
	DeviceType string                `json:"device_type" validate:"omitempty,oneof=smartphone tablet tv computer home_electronics"`
	Brand      string                `json:"brand" validate:"required,min=1,max=100"`
	Model      string                `json:"model" validate:"required,min=1,max=200"`
	Metadata   DeviceMetadataPayload `json:"metadata"`
	IMEI       string                `json:"imei" validate:"omitempty,len=15,numeric"`
}

// UpdateDeviceRequest is the request body for updating a device.
type UpdateDeviceRequest struct {
	DeviceType string                 `json:"device_type" validate:"omitempty,oneof=smartphone tablet tv computer home_electronics"`
	Brand      string                 `json:"brand" validate:"required,min=1,max=100"`
	Model      string                 `json:"model" validate:"required,min=1,max=200"`
	Metadata   *DeviceMetadataPayload `json:"metadata,omitempty"`
	IMEI       string                 `json:"imei" validate:"omitempty,len=15,numeric"`
}

// AddSubscriptionDeviceRequest adds a device to an existing subscription.
// Brand is optional (non-phones use a single combined name in model).
type AddSubscriptionDeviceRequest struct {
	DeviceType         string                `json:"device_type" validate:"omitempty,oneof=smartphone tablet tv computer game_console home_electronics"`
	Brand              string                `json:"brand" validate:"omitempty,max=100"`
	Model              string                `json:"model" validate:"required,min=1,max=200"`
	Metadata           DeviceMetadataPayload `json:"metadata"`
	IMEI               string                `json:"imei" validate:"omitempty,len=15,numeric"`
	VerificationPhotos []string              `json:"verification_photos"`
	VerificationVideo  string                `json:"verification_video"`
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

	device, appErr := h.svc.Create(
		r.Context(),
		ac,
		domain.NormalizeDeviceType(req.DeviceType),
		req.Brand,
		req.Model,
		req.IMEI,
		req.Metadata.ToDomain(),
	)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, device)
}

// AddToSubscription adds a device to an existing subscription (free, up to the
// plan's per-type caps). POST /api/v1/subscriptions/{id}/devices.
func (h *DeviceHandler) AddToSubscription(w http.ResponseWriter, r *http.Request) {
	subID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid subscription ID"))
		return
	}
	var req AddSubscriptionDeviceRequest
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
	device, appErr := h.svc.AddToSubscription(
		r.Context(), ac, subID,
		domain.NormalizeDeviceType(req.DeviceType),
		req.Brand, req.Model, req.IMEI, req.Metadata.ToDomain(),
		req.VerificationPhotos, req.VerificationVideo,
	)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusCreated, device)
}

// ListSubscriptionDevices lists devices attached to a subscription, so the UI
// can show remaining slots. GET /api/v1/subscriptions/{id}/devices.
func (h *DeviceHandler) ListSubscriptionDevices(w http.ResponseWriter, r *http.Request) {
	subID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid subscription ID"))
		return
	}
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}
	devices, appErr := h.svc.ListSubscriptionDevices(r.Context(), ac, subID)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}
	WriteSuccess(w, r, http.StatusOK, devices)
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

	var metadata *domain.DeviceMetadata
	if req.Metadata != nil {
		value := req.Metadata.ToDomain()
		metadata = &value
	}

	device, appErr := h.svc.Update(
		r.Context(),
		ac,
		id,
		domain.NormalizeDeviceType(req.DeviceType),
		req.Brand,
		req.Model,
		req.IMEI,
		metadata,
	)
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
