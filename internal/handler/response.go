// Package handler provides HTTP request handlers.
package handler

import (
	"net/http"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/respond"
)

// WriteJSON delegates to respond.JSON.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	respond.JSON(w, status, data)
}

// WriteSuccess delegates to respond.Success.
func WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any) {
	respond.Success(w, r, status, data)
}

// WriteError delegates to respond.Error.
func WriteError(w http.ResponseWriter, r *http.Request, appErr *domain.AppError) {
	respond.Error(w, r, appErr)
}

// DecodeJSON delegates to respond.DecodeJSON.
func DecodeJSON(r *http.Request, target any) error {
	return respond.DecodeJSON(r, target)
}
