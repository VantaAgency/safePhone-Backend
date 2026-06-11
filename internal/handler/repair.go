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

// RepairHandler handles HTTP requests for repair bookings.
type RepairHandler struct {
	svc      *service.RepairService
	validate *validator.Validate
}

// NewRepairHandler creates a new repair handler.
func NewRepairHandler(svc *service.RepairService) *RepairHandler {
	return &RepairHandler{svc: svc, validate: validator.New()}
}

// CreateRepairBookingRequest is the request body for booking a repair.
type CreateRepairBookingRequest struct {
	DeviceBrand   string  `json:"device_brand" validate:"required,min=1,max=100"`
	DeviceModel   string  `json:"device_model" validate:"required,min=1,max=150"`
	RepairType    string  `json:"repair_type" validate:"required,min=1,max=100"`
	ServiceMode   string  `json:"service_mode" validate:"required,oneof=center home"`
	CenterID      *string `json:"center_id" validate:"omitempty,min=1,max=100"`
	PreferredDate string  `json:"preferred_date" validate:"required"`
	PreferredTime string  `json:"preferred_time" validate:"required"`
	CustomerName  string  `json:"customer_name" validate:"required,min=2,max=200"`
	CustomerPhone string  `json:"customer_phone" validate:"required,min=6,max=30"`
}

// LookupRepairBookingRequest is the request body for public repair tracking.
type LookupRepairBookingRequest struct {
	Reference     string `json:"reference" validate:"required,min=4,max=20"`
	CustomerPhone string `json:"customer_phone" validate:"required,min=6,max=30"`
}

// UpdateRepairStatusRequest is the request body for admin repair status updates.
type UpdateRepairStatusRequest struct {
	Status        string  `json:"status" validate:"required,oneof=accepted rejected scheduled in_progress completed cancelled"`
	ScheduledDate *string `json:"scheduled_date" validate:"omitempty"`
	ScheduledTime *string `json:"scheduled_time" validate:"omitempty"`
}

// UpdateRepairAmountRequest is the request body for admin repair amount updates.
type UpdateRepairAmountRequest struct {
	RepairAmountMinor int `json:"repair_amount_minor" validate:"min=0"`
}

// CreateBooking handles POST /api/v1/repairs (public).
func (h *RepairHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req CreateRepairBookingRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	// Optionally associate with authenticated user when optional auth middleware injects it.
	var ac *auth.AuthContext
	if authContext, err := auth.GetAuthContext(r.Context()); err == nil {
		ac = authContext
	}

	booking, appErr := h.svc.CreateBooking(
		r.Context(),
		ac,
		req.DeviceBrand, req.DeviceModel, req.RepairType, req.ServiceMode,
		req.CenterID,
		req.PreferredDate, req.PreferredTime,
		req.CustomerName, req.CustomerPhone,
	)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, booking)
}

// LookupBooking handles POST /api/v1/repairs/lookup (public).
func (h *RepairHandler) LookupBooking(w http.ResponseWriter, r *http.Request) {
	var req LookupRepairBookingRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	booking, appErr := h.svc.LookupByReference(r.Context(), req.Reference, req.CustomerPhone)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, booking)
}

// ListMine handles GET /api/v1/repairs/mine.
func (h *RepairHandler) ListMine(w http.ResponseWriter, r *http.Request) {
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

	bookings, appErr := h.svc.ListMine(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, bookings)
}

// AdminList handles GET /api/v1/admin/repairs.
func (h *RepairHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	market, appErr := parseMarketQuery(r.URL.Query().Get("market"))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	bookings, appErr := h.svc.AdminList(r.Context(), ac, statusPtr, search, market, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, bookings)
}

// AdminGet handles GET /api/v1/admin/repairs/{id}.
func (h *RepairHandler) AdminGet(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid repair request ID"))
		return
	}

	booking, appErr := h.svc.AdminGet(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, booking)
}

// AdminAccept handles POST /api/v1/admin/repairs/{id}/accept.
func (h *RepairHandler) AdminAccept(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid repair request ID"))
		return
	}

	booking, appErr := h.svc.AdminAccept(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, booking)
}

// AdminReject handles POST /api/v1/admin/repairs/{id}/reject.
func (h *RepairHandler) AdminReject(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid repair request ID"))
		return
	}

	booking, appErr := h.svc.AdminReject(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, booking)
}

// AdminUpdateStatus handles PUT /api/v1/admin/repairs/{id}/status.
func (h *RepairHandler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid repair request ID"))
		return
	}

	var req UpdateRepairStatusRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	booking, appErr := h.svc.AdminUpdateStatus(r.Context(), ac, id, req.Status, req.ScheduledDate, req.ScheduledTime)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, booking)
}

// AdminUpdateAmount handles PUT /api/v1/admin/repairs/{id}/amount.
func (h *RepairHandler) AdminUpdateAmount(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid repair request ID"))
		return
	}

	var req UpdateRepairAmountRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	booking, appErr := h.svc.AdminUpdateAmount(r.Context(), ac, id, req.RepairAmountMinor)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, booking)
}
