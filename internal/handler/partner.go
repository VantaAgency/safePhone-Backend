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

// PartnerHandler handles HTTP requests for the partner domain.
type PartnerHandler struct {
	svc      *service.PartnerService
	validate *validator.Validate
}

// NewPartnerHandler creates a new partner handler.
func NewPartnerHandler(svc *service.PartnerService) *PartnerHandler {
	return &PartnerHandler{svc: svc, validate: validator.New()}
}

// GetProfile handles GET /api/v1/partner/profile.
func (h *PartnerHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	profile, appErr := h.svc.GetProfile(r.Context(), ac)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, profile)
}

// ListClients handles GET /api/v1/partner/clients.
func (h *PartnerHandler) ListClients(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	clients, appErr := h.svc.ListClients(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, clients)
}

// CreateClientRequest is the request body for adding a partner client.
type CreateClientRequest struct {
	ClientName  string  `json:"client_name" validate:"required,min=2,max=200"`
	ClientPhone string  `json:"client_phone" validate:"omitempty,max=30"`
	PlanID      *string `json:"plan_id" validate:"omitempty,uuid"`
}

// CreateClient handles POST /api/v1/partner/clients.
func (h *PartnerHandler) CreateClient(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	var req CreateClientRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	var planID *uuid.UUID
	if req.PlanID != nil {
		id, parseErr := uuid.Parse(*req.PlanID)
		if parseErr != nil {
			WriteError(w, r, domain.BadRequest("invalid plan_id"))
			return
		}
		planID = &id
	}

	client, appErr := h.svc.CreateClient(r.Context(), ac, req.ClientName, req.ClientPhone, planID)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, client)
}

// RefreshInvitation handles POST /api/v1/partner/clients/{id}/refresh-invitation.
func (h *PartnerHandler) RefreshInvitation(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	clientID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid client id"))
		return
	}

	client, appErr := h.svc.RefreshInvitation(r.Context(), ac, clientID)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, client)
}

// GetInvitation handles GET /api/v1/partner-invitations/{token}.
func (h *PartnerHandler) GetInvitation(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		WriteError(w, r, domain.BadRequest("missing invitation token"))
		return
	}

	details, appErr := h.svc.GetInvitationDetails(r.Context(), token)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, details)
}

// ClaimInvitation handles POST /api/v1/partner-invitations/{token}/claim.
func (h *PartnerHandler) ClaimInvitation(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		WriteError(w, r, domain.BadRequest("missing invitation token"))
		return
	}

	details, appErr := h.svc.ClaimInvitation(r.Context(), ac, token)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, details)
}

// UpdateClientStatusRequest is the request body for updating a client status.
type UpdateClientStatusRequest struct {
	Status string  `json:"status" validate:"required,oneof=draft invited account_created payment_pending active expired cancelled failed"`
	PlanID *string `json:"plan_id" validate:"omitempty,uuid"`
}

// UpdateClientStatus handles PATCH /api/v1/partner-clients/{id}/status (public).
func (h *PartnerHandler) UpdateClientStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	clientID, err := uuid.Parse(idStr)
	if err != nil {
		WriteError(w, r, domain.BadRequest("invalid client id"))
		return
	}

	var req UpdateClientStatusRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	var planID *uuid.UUID
	if req.PlanID != nil {
		id, parseErr := uuid.Parse(*req.PlanID)
		if parseErr != nil {
			WriteError(w, r, domain.BadRequest("invalid plan_id"))
			return
		}
		planID = &id
	}

	if appErr := h.svc.UpdateClientStatus(r.Context(), clientID, req.Status, planID); appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, map[string]string{"status": req.Status})
}

// ListSales handles GET /api/v1/partner/sales.
func (h *PartnerHandler) ListSales(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	sales, appErr := h.svc.ListSales(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, sales)
}

// ListPayouts handles GET /api/v1/partner/payouts.
func (h *PartnerHandler) ListPayouts(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	payouts, appErr := h.svc.ListPayouts(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, payouts)
}

// ListAllPartners handles GET /api/v1/admin/partners.
func (h *PartnerHandler) ListAllPartners(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	partners, appErr := h.svc.ListAll(r.Context(), ac, limit, offset)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, partners)
}
