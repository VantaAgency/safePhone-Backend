package handler

import (
	"net/http"
	"strconv"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// AdminHandler handles admin-level HTTP requests.
type AdminHandler struct {
	svc *service.AdminService
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
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

	customers, appErr := h.svc.ListCustomers(r.Context(), ac, search, limit, offset)
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

	payments, appErr := h.svc.ListPayments(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, payments)
}
