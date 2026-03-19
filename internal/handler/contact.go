package handler

import (
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// CreateContactRequest is the request body for the contact form.
type CreateContactRequest struct {
	Name    string  `json:"name" validate:"required,min=1,max=200"`
	Email   string  `json:"email" validate:"required,email,max=255"`
	Subject *string `json:"subject" validate:"omitempty,max=500"`
	Message string  `json:"message" validate:"required,min=1"`
}

// ContactHandler handles contact form HTTP requests.
type ContactHandler struct {
	svc      *service.ContactService
	validate *validator.Validate
}

// NewContactHandler creates a new contact handler.
func NewContactHandler(svc *service.ContactService) *ContactHandler {
	return &ContactHandler{svc: svc, validate: validator.New()}
}

// Submit handles POST /api/v1/contact.
func (h *ContactHandler) Submit(w http.ResponseWriter, r *http.Request) {
	var req CreateContactRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	msg, appErr := h.svc.Submit(r.Context(), req.Name, req.Email, req.Subject, req.Message)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, msg)
}
