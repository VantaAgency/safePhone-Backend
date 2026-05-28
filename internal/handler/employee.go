package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// EmployeeUpdateClaimStatusRequest is the request body for employee claim triage.
type EmployeeUpdateClaimStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=review"`
}

// UpsertOperationalFollowUpRequest is the request body for follow-up updates.
type UpsertOperationalFollowUpRequest struct {
	EntityType    string     `json:"entity_type" validate:"required,oneof=client subscription claim repair"`
	EntityID      string     `json:"entity_id" validate:"required,uuid"`
	Reason        *string    `json:"reason" validate:"omitempty,max=255"`
	Status        string     `json:"status" validate:"required,oneof=to_contact contacted awaiting_response resolved"`
	NextAction    *string    `json:"next_action" validate:"omitempty,max=2000"`
	LastContactAt *time.Time `json:"last_contact_at"`
}

// CreateOperationalNoteRequest is the request body for internal notes.
type CreateOperationalNoteRequest struct {
	EntityType string `json:"entity_type" validate:"required,oneof=client subscription claim repair"`
	EntityID   string `json:"entity_id" validate:"required,uuid"`
	Body       string `json:"body" validate:"required,max=5000"`
}

// EmployeeHandler handles employee operational endpoints.
type EmployeeHandler struct {
	svc      *service.EmployeeService
	validate *validator.Validate
}

// NewEmployeeHandler creates a new employee handler.
func NewEmployeeHandler(svc *service.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Overview handles GET /api/v1/employee/overview.
func (h *EmployeeHandler) Overview(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	overview, appErr := h.svc.GetOverview(r.Context(), ac)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, overview)
}

// ListClients handles GET /api/v1/employee/clients.
func (h *EmployeeHandler) ListClients(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	items, appErr := h.svc.ListClients(r.Context(), ac, r.URL.Query().Get("search"), parseLimit(r, 50, 100), parseOffset(r))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, items)
}

// GetClient handles GET /api/v1/employee/clients/{id}.
func (h *EmployeeHandler) GetClient(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	clientID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid client ID")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	item, serviceErr := h.svc.GetClient(r.Context(), ac, clientID)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// ListPaymentFollowUps handles GET /api/v1/employee/payment-follow-ups.
func (h *EmployeeHandler) ListPaymentFollowUps(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	items, appErr := h.svc.ListPaymentFollowUps(r.Context(), ac, r.URL.Query().Get("search"), parseLimit(r, 50, 100), parseOffset(r))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, items)
}

// ListClaims handles GET /api/v1/employee/claims.
func (h *EmployeeHandler) ListClaims(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	status, appErr := parseClaimStatusQuery(r.URL.Query().Get("status"))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	items, serviceErr := h.svc.ListClaims(r.Context(), ac, status, r.URL.Query().Get("search"), parseLimit(r, 50, 100), parseOffset(r))
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, items)
}

// GetClaim handles GET /api/v1/employee/claims/{id}.
func (h *EmployeeHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	claimID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid claim ID")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	item, serviceErr := h.svc.GetClaim(r.Context(), ac, claimID)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// UpdateClaimStatus handles PATCH /api/v1/employee/claims/{id}/status.
func (h *EmployeeHandler) UpdateClaimStatus(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	claimID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid claim ID")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	var req EmployeeUpdateClaimStatusRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	item, serviceErr := h.svc.UpdateClaimStatus(r.Context(), ac, claimID, domain.ClaimStatus(req.Status))
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// ListRepairs handles GET /api/v1/employee/repairs.
func (h *EmployeeHandler) ListRepairs(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	status, appErr := parseRepairStatusQuery(r.URL.Query().Get("status"))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	items, serviceErr := h.svc.ListRepairs(r.Context(), ac, status, r.URL.Query().Get("search"), parseLimit(r, 50, 100), parseOffset(r))
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, items)
}

// GetRepair handles GET /api/v1/employee/repairs/{id}.
func (h *EmployeeHandler) GetRepair(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	repairID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid repair request ID")
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	item, serviceErr := h.svc.GetRepair(r.Context(), ac, repairID)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// UpdateRepairStatus handles PUT /api/v1/employee/repairs/{id}/status.
func (h *EmployeeHandler) UpdateRepairStatus(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	repairID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid repair request ID")
	if appErr != nil {
		WriteError(w, r, appErr)
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

	item, serviceErr := h.svc.UpdateRepairStatus(r.Context(), ac, repairID, req.Status, req.ScheduledDate, req.ScheduledTime)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// UpdateRepairAmount handles PUT /api/v1/employee/repairs/{id}/amount.
func (h *EmployeeHandler) UpdateRepairAmount(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	repairID, appErr := parseUUIDParam(chi.URLParam(r, "id"), "invalid repair request ID")
	if appErr != nil {
		WriteError(w, r, appErr)
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

	item, serviceErr := h.svc.UpdateRepairAmount(r.Context(), ac, repairID, req.RepairAmountMinor)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// ListTasks handles GET /api/v1/employee/tasks.
func (h *EmployeeHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	items, appErr := h.svc.ListTasks(r.Context(), ac, parseLimit(r, 50, 200), parseOffset(r))
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, items)
}

// GetFollowUp handles GET /api/v1/employee/follow-ups.
func (h *EmployeeHandler) GetFollowUp(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	entityType, entityID, appErr := parseOperationalEntityScope(r)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	item, serviceErr := h.svc.GetFollowUp(r.Context(), ac, entityType, entityID)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// UpsertFollowUp handles PUT /api/v1/employee/follow-ups.
func (h *EmployeeHandler) UpsertFollowUp(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	var req UpsertOperationalFollowUpRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	entityID, _ := uuid.Parse(req.EntityID)
	item, serviceErr := h.svc.UpsertFollowUp(
		r.Context(),
		ac,
		domain.OperationalEntityType(req.EntityType),
		entityID,
		req.Reason,
		domain.FollowUpStatus(req.Status),
		req.NextAction,
		req.LastContactAt,
	)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, item)
}

// ListNotes handles GET /api/v1/employee/notes.
func (h *EmployeeHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	entityType, entityID, appErr := parseOperationalEntityScope(r)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	items, serviceErr := h.svc.ListNotes(r.Context(), ac, entityType, entityID, parseLimit(r, 50, 100), parseOffset(r))
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, items)
}

// CreateNote handles POST /api/v1/employee/notes.
func (h *EmployeeHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	ac, ok := h.requireAuthContext(w, r)
	if !ok {
		return
	}

	var req CreateOperationalNoteRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	entityID, _ := uuid.Parse(req.EntityID)
	item, serviceErr := h.svc.CreateNote(r.Context(), ac, domain.OperationalEntityType(req.EntityType), entityID, req.Body)
	if serviceErr != nil {
		WriteError(w, r, serviceErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, item)
}

func (h *EmployeeHandler) requireAuthContext(w http.ResponseWriter, r *http.Request) (*auth.AuthContext, bool) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return nil, false
	}
	return ac, true
}

func parseUUIDParam(raw, message string) (uuid.UUID, *domain.AppError) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, domain.BadRequest(message)
	}
	return id, nil
}

func parseOperationalEntityScope(r *http.Request) (domain.OperationalEntityType, uuid.UUID, *domain.AppError) {
	entityType := strings.TrimSpace(r.URL.Query().Get("entity_type"))
	entityID := strings.TrimSpace(r.URL.Query().Get("entity_id"))
	if entityType == "" || entityID == "" {
		return "", uuid.Nil, domain.BadRequest("entity_type and entity_id are required")
	}

	switch domain.OperationalEntityType(entityType) {
	case domain.OperationalEntityTypeClient, domain.OperationalEntityTypeSubscription, domain.OperationalEntityTypeClaim, domain.OperationalEntityTypeRepair:
	default:
		return "", uuid.Nil, domain.BadRequest("invalid entity type")
	}

	parsedID, err := uuid.Parse(entityID)
	if err != nil {
		return "", uuid.Nil, domain.BadRequest("invalid entity ID")
	}

	return domain.OperationalEntityType(entityType), parsedID, nil
}

func parseClaimStatusQuery(raw string) (*string, *domain.AppError) {
	status := strings.TrimSpace(raw)
	if status == "" {
		return nil, nil
	}

	switch domain.ClaimStatus(status) {
	case domain.ClaimStatusPending, domain.ClaimStatusReview, domain.ClaimStatusApproved, domain.ClaimStatusRejected, domain.ClaimStatusSettled:
		return &status, nil
	default:
		return nil, domain.BadRequest("invalid claim status")
	}
}

func parseRepairStatusQuery(raw string) (*string, *domain.AppError) {
	status := strings.TrimSpace(raw)
	if status == "" {
		return nil, nil
	}

	switch status {
	case domain.RepairStatusPending, domain.RepairStatusAccepted, domain.RepairStatusRejected, domain.RepairStatusScheduled, domain.RepairStatusInProgress, domain.RepairStatusCompleted, domain.RepairStatusCancelled:
		return &status, nil
	default:
		return nil, domain.BadRequest("invalid repair status")
	}
}

func parseLimit(r *http.Request, defaultValue, maxValue int) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		return defaultValue
	}
	if limit > maxValue {
		return maxValue
	}
	return limit
}

func parseOffset(r *http.Request) int {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		return 0
	}
	return offset
}
