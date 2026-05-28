package domain

import (
	"fmt"
	"net/http"
)

// Error code constants used across the application.
const (
	CodeNotFound           = "NOT_FOUND"
	CodeValidationFailed   = "VALIDATION_FAILED"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeConflict           = "CONFLICT"
	CodeInternalError      = "INTERNAL_ERROR"
	CodeRateLimited        = "RATE_LIMITED"
	CodeBadRequest         = "BAD_REQUEST"
	CodePaymentGateway     = "PAYMENT_GATEWAY_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// AppError is the standard error type returned by services.
// Handlers unwrap it to HTTP responses. The Internal field is logged server-side only.
type AppError struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	HTTPStatus int               `json:"-"`
	Internal   error             `json:"-"`
	Fields     map[string]string `json:"fields,omitempty"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %s (internal: %v)", e.Code, e.Message, e.Internal)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the internal error for errors.Is/As compatibility.
func (e *AppError) Unwrap() error {
	return e.Internal
}

// NotFound creates a not-found error.
func NotFound(resource string) *AppError {
	return &AppError{
		Code:       CodeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		HTTPStatus: http.StatusNotFound,
	}
}

// ValidationFailed creates a validation error with field details.
func ValidationFailed(message string, fields map[string]string) *AppError {
	return &AppError{
		Code:       CodeValidationFailed,
		Message:    message,
		HTTPStatus: http.StatusUnprocessableEntity,
		Fields:     fields,
	}
}

// Unauthorized creates an authentication error.
func Unauthorized(message string) *AppError {
	return &AppError{
		Code:       CodeUnauthorized,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

// Forbidden creates an authorization error.
func Forbidden(message string) *AppError {
	return &AppError{
		Code:       CodeForbidden,
		Message:    message,
		HTTPStatus: http.StatusForbidden,
	}
}

// Conflict creates a conflict error (e.g., duplicate IMEI).
func Conflict(message string) *AppError {
	return &AppError{
		Code:       CodeConflict,
		Message:    message,
		HTTPStatus: http.StatusConflict,
	}
}

// BadRequest creates a bad request error.
func BadRequest(message string) *AppError {
	return &AppError{
		Code:       CodeBadRequest,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

// InternalError creates an internal error, wrapping the original for logging.
func InternalError(internal error) *AppError {
	return &AppError{
		Code:       CodeInternalError,
		Message:    "an internal error occurred",
		HTTPStatus: http.StatusInternalServerError,
		Internal:   internal,
	}
}

// PaymentGatewayError creates a payment gateway error, wrapping the original for logging.
func PaymentGatewayError(internal error) *AppError {
	return &AppError{
		Code:       CodePaymentGateway,
		Message:    "payment gateway error",
		HTTPStatus: http.StatusBadGateway,
		Internal:   internal,
	}
}

// Internal creates an internal error with a plain message. Prefer
// InternalError when you have a wrapped error to attach.
func Internal(message string) *AppError {
	return &AppError{
		Code:       CodeInternalError,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// ServiceUnavailable indicates an integration is disabled or offline
// (e.g. Stripe not configured).
func ServiceUnavailable(message string) *AppError {
	return &AppError{
		Code:       CodeServiceUnavailable,
		Message:    message,
		HTTPStatus: http.StatusServiceUnavailable,
	}
}

// RateLimited creates a rate-limit error.
func RateLimited() *AppError {
	return &AppError{
		Code:       CodeRateLimited,
		Message:    "too many requests",
		HTTPStatus: http.StatusTooManyRequests,
	}
}
