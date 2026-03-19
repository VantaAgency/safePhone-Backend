// Package respond provides HTTP JSON response helpers used by handlers and middleware.
package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// SuccessResponse wraps data with metadata.
type SuccessResponse struct {
	Data any      `json:"data"`
	Meta MetaInfo `json:"meta"`
}

// ErrorResponse wraps an error with metadata.
type ErrorResponse struct {
	Error ErrorInfo `json:"error"`
}

// MetaInfo provides request metadata.
type MetaInfo struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

// ErrorInfo provides machine-readable error details.
type ErrorInfo struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// Success writes a successful JSON response with metadata.
func Success(w http.ResponseWriter, r *http.Request, status int, data any) {
	requestID := r.Header.Get("X-Request-ID")
	JSON(w, status, SuccessResponse{
		Data: data,
		Meta: MetaInfo{
			RequestID: requestID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// Error writes an error JSON response. Logs internal errors server-side.
func Error(w http.ResponseWriter, r *http.Request, appErr *domain.AppError) {
	if appErr.Internal != nil {
		slog.Error("internal error",
			"code", appErr.Code,
			"message", appErr.Message,
			"internal", appErr.Internal.Error(),
			"request_id", r.Header.Get("X-Request-ID"),
			"path", r.URL.Path,
		)
	}

	requestID := r.Header.Get("X-Request-ID")
	JSON(w, appErr.HTTPStatus, ErrorResponse{
		Error: ErrorInfo{
			Code:      appErr.Code,
			Message:   appErr.Message,
			RequestID: requestID,
			Fields:    appErr.Fields,
		},
	})
}

// DecodeJSON decodes a JSON request body into the target, rejecting unknown fields.
func DecodeJSON(r *http.Request, target any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}
