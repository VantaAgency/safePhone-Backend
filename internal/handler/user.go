package handler

import (
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// UpdateProfileRequest is the request body for updating the user's profile.
type UpdateProfileRequest struct {
	Phone string `json:"phone" validate:"required,min=1,max=30"`
}

// UserHandler handles user profile HTTP requests.
type UserHandler struct {
	svc      *service.UserService
	validate *validator.Validate
}

// NewUserHandler creates a new user handler.
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// UpdateProfile handles PATCH /api/v1/users/me.
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ac, err := auth.GetAuthContext(r.Context())
	if err != nil {
		WriteError(w, r, domain.Unauthorized("authentication required"))
		return
	}

	var req UpdateProfileRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, domain.BadRequest("invalid request body"))
		return
	}
	if err := h.validate.Struct(req); err != nil {
		WriteError(w, r, domain.ValidationFailed("validation failed", nil))
		return
	}

	user, appErr := h.svc.UpdatePhone(r.Context(), ac, req.Phone)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusOK, user)
}
