package handler

import (
	"net/http"

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
	DeviceType    string `json:"device_type" validate:"required,min=1,max=100"`
	RepairType    string `json:"repair_type" validate:"required,min=1,max=100"`
	LocationID    string `json:"location_id" validate:"required,min=1,max=100"`
	BookingDate   string `json:"booking_date" validate:"required"`
	BookingTime   string `json:"booking_time" validate:"required"`
	CustomerName  string `json:"customer_name" validate:"required,min=2,max=200"`
	CustomerPhone string `json:"customer_phone" validate:"required,min=6,max=30"`
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

	// Optionally associate with authenticated user if JWT is present
	var orgID, userID *uuid.UUID
	if ac, err := auth.GetAuthContext(r.Context()); err == nil {
		orgID = &ac.OrgID
		userID = &ac.UserID
	}

	booking, appErr := h.svc.CreateBooking(
		r.Context(),
		orgID, userID,
		req.DeviceType, req.RepairType, req.LocationID,
		req.BookingDate, req.BookingTime,
		req.CustomerName, req.CustomerPhone,
	)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, booking)
}
