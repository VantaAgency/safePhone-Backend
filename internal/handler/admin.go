package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// UpdateEmployeeStatusRequest is the request body for admin employee status updates.
type UpdateEmployeeStatusRequest struct {
	Status          string  `json:"status" validate:"required,oneof=active inactive suspended"`
	SuspendedReason *string `json:"suspended_reason" validate:"omitempty,max=2000"`
}

// AdminHandler handles admin-level HTTP requests.
type AdminHandler struct {
	svc      *service.AdminService
	validate *validator.Validate
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Stats returns aggregate platform statistics.
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	stats, appErr := h.svc.GetStats(r.Context(), ac)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, stats)
}

// ListCustomers returns org customers with optional search.
func (h *AdminHandler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	search := r.URL.Query().Get("search")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	market, appErr := parseMarketQuery(r.URL.Query().Get("market"))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	customers, appErr := h.svc.ListCustomers(r.Context(), ac, search, market, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, customers)
}

// ListPayments returns all payments in the org.
func (h *AdminHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
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

	payments, appErr := h.svc.ListPayments(r.Context(), ac, market, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, payments)
}

// ListEmployees returns admin employee management rows.
func (h *AdminHandler) ListEmployees(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	status, appErr := parseEmployeeAccountStatusQuery(r.URL.Query().Get("status"))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	items, serviceErr := h.svc.ListEmployees(
		r.Context(),
		ac,
		r.URL.Query().Get("search"),
		status,
		r.URL.Query().Get("sort"),
		parseLimit(r, 50, 100),
		parseOffset(r),
	)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, items)
}

// GetEmployee returns a single admin employee detail.
func (h *AdminHandler) GetEmployee(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	userID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid employee ID")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	item, serviceErr := h.svc.GetEmployee(r.Context(), ac, userID)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// UpdateEmployeeStatus updates an employee account status.
func (h *AdminHandler) UpdateEmployeeStatus(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	userID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid employee ID")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	var req UpdateEmployeeStatusRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	item, serviceErr := h.svc.UpdateEmployeeStatus(
		r.Context(),
		ac,
		userID,
		domain.EmployeeAccountStatus(req.Status),
		req.SuspendedReason,
	)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// parseMarketQuery validates an optional ?market= filter on admin list
// endpoints. Empty or "all" means no filter; otherwise the value must be a
// known market (SN/US). Returns "" for "no filter" so it can be passed
// straight to the repositories, where ($n = '' OR market = $n) matches all
// rows when the value is empty.
func parseMarketQuery(raw string) (string, *domain.AppError) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "all") {
		return "", nil
	}
	market := strings.ToUpper(raw)
	if !domain.IsValidMarket(market) {
		return "", domain.BadRequest("invalid market filter")
	}
	return market, nil
}

func parseEmployeeAccountStatusQuery(raw string) (*domain.EmployeeAccountStatus, *domain.AppError) {
	switch raw {
	case "":
		return nil, nil
	case string(domain.EmployeeAccountStatusActive),
		string(domain.EmployeeAccountStatusInactive),
		string(domain.EmployeeAccountStatusSuspended):
		status := domain.EmployeeAccountStatus(raw)
		return &status, nil
	default:
		return nil, domain.BadRequest("invalid employee status")
	}
}
