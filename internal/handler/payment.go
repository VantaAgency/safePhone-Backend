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

// CreatePaymentRequest is the request body for initiating a payment.
// It atomically creates a device, subscription, and payment in one call.
type CreatePaymentRequest struct {
	// Device fields
	Brand string `json:"brand" validate:"required,min=1,max=100"`
	Model string `json:"model" validate:"required,min=1,max=200"`
	IMEI  string `json:"imei" validate:"omitempty,len=15,numeric"`

	// Subscription fields
	PlanID       string `json:"plan_id" validate:"required,uuid"`
	BillingCycle string `json:"billing_cycle" validate:"required,oneof=monthly annual"`

	// Payment fields
	IdempotencyKey string `json:"idempotency_key" validate:"omitempty,max=100"`
}

// PaymentHandler handles payment-related HTTP requests.
type PaymentHandler struct {
	svc      *service.PaymentService
	validate *validator.Validate
}

// NewPaymentHandler creates a new payment handler.
func NewPaymentHandler(svc *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Create initiates a new payment with atomic device+subscription creation.
func (h *PaymentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentRequest
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

	planID, _ := uuid.Parse(req.PlanID)

	var idempKey *string
	if req.IdempotencyKey != "" {
		idempKey = &req.IdempotencyKey
	}

	result, appErr := h.svc.Create(
		r.Context(), ac,
		req.Brand, req.Model, req.IMEI,
		planID, req.BillingCycle, idempKey,
	)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, result)
}

// List returns the authenticated user's payments.
func (h *PaymentHandler) List(w http.ResponseWriter, r *http.Request) {
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

	payments, appErr := h.svc.List(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, payments)
}

// Get returns a single payment by ID.
func (h *PaymentHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid payment ID"))
		return
	}

	payment, appErr := h.svc.Get(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, payment)
}

// GetCheckout returns the canonical checkout state for a payment attempt.
func (h *PaymentHandler) GetCheckout(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid payment ID"))
		return
	}

	result, appErr := h.svc.GetCheckout(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, result)
}

// Resume reuses or recreates a checkout session for an existing payment.
func (h *PaymentHandler) Resume(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid payment ID"))
		return
	}

	result, appErr := h.svc.Resume(r.Context(), ac, id)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, result)
}
