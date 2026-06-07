package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-playground/validator/v10"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
	stripepkg "github.com/cherif-safephone/safephone-backend/internal/stripe"
)

// isSafeHTTPURL accepts only http and https. The validator's "url" rule
// parses anything net/url can parse, including javascript: and data:.
// Verification URLs are stored and later rendered as <a href> in the
// admin tab, so we tighten the gate at the API boundary as defense in
// depth on top of the frontend safeHttpUrl helper.
func isSafeHTTPURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// StripeCheckoutRequest starts a Stripe Checkout session for a US plan.
type StripeCheckoutRequest struct {
	PlanSlug string `json:"plan_slug" validate:"required,min=3,max=64"`
}

// RegisterUSDeviceRequest attaches a device to the user's pending US
// subscription. Plans v2 added the verification proof fields — the
// handler enforces a 2-photo minimum + 1 video; the service layer
// stores them onto the device row and the admin reviews them via the
// Verifications tab before the subscription leaves pending_verification.
type RegisterUSDeviceRequest struct {
	// Brand required only for smartphones — enforced per device type in the
	// service. Non-phones send an empty brand and carry the name in Model.
	Brand        string   `json:"brand" validate:"omitempty,max=100"`
	Model        string   `json:"model" validate:"required,min=1,max=200"`
	IMEI         string   `json:"imei" validate:"omitempty,len=15,numeric"`
	DeviceType   string   `json:"device_type" validate:"omitempty,oneof=smartphone tablet computer game_console tv"`
	SerialNumber string   `json:"serial_number" validate:"omitempty,max=120"`
	Photos       []string `json:"photos" validate:"omitempty,dive,url"`
	Video        string   `json:"video" validate:"omitempty,url"`
}

// StripeHandler exposes Stripe checkout, webhook, and US device registration.
type StripeHandler struct {
	svc      *service.StripeService
	client   *stripepkg.Client
	validate *validator.Validate
}

// NewStripeHandler builds the handler. svc and client may be nil when Stripe
// isn't configured — endpoints respond with 503 in that case.
func NewStripeHandler(svc *service.StripeService, client *stripepkg.Client) *StripeHandler {
	return &StripeHandler{svc: svc, client: client, validate: validator.New()}
}

func (h *StripeHandler) enabled() bool {
	return h != nil && h.svc != nil && h.svc.Enabled() && h.client != nil
}

// CreateCheckout creates a Stripe Checkout session for the authenticated user.
func (h *StripeHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	if !h.enabled() {
		WriteError(w, r, domain.ServiceUnavailable("Stripe payments are not configured"))
		return
	}

	var req StripeCheckoutRequest
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

	result, appErr := h.svc.CreateCheckout(r.Context(), ac, req.PlanSlug)
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, result)
}

// RegisterDevice attaches a device to the user's pending US subscription.
func (h *StripeHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	if !h.enabled() {
		WriteError(w, r, domain.ServiceUnavailable("Stripe payments are not configured"))
		return
	}

	var req RegisterUSDeviceRequest
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

	// Plans v2 verification gate. Empty arrays are allowed (legacy
	// register-device form without verification). The required photo count
	// is per device type (smartphone 2, others 1) — use the same
	// RequiredVerificationPhotos source as the service to avoid drift.
	if n := domain.RequiredVerificationPhotos(domain.NormalizeDeviceType(req.DeviceType)); len(req.Photos) > 0 && len(req.Photos) < n {
		WriteError(w, r, domain.ValidationFailed("insufficient verification photos", map[string]string{
			"photos": "more photo URLs are required for this device type",
		}))
		return
	}
	for _, p := range req.Photos {
		if !isSafeHTTPURL(p) {
			WriteError(w, r, domain.ValidationFailed("verification photo URLs must be http(s)", map[string]string{
				"photos": "only http(s) URLs are accepted",
			}))
			return
		}
	}
	if req.Video != "" && !isSafeHTTPURL(req.Video) {
		WriteError(w, r, domain.ValidationFailed("verification video URL must be http(s)", map[string]string{
			"video": "only http(s) URLs are accepted",
		}))
		return
	}

	result, appErr := h.svc.RegisterDevice(r.Context(), ac, service.RegisterDeviceParams{
		Brand:        req.Brand,
		Model:        req.Model,
		IMEI:         req.IMEI,
		DeviceType:   req.DeviceType,
		SerialNumber: req.SerialNumber,
		Photos:       req.Photos,
		Video:        req.Video,
	})
	if appErr != nil {
		WriteError(w, r, appErr)
		return
	}

	WriteSuccess(w, r, http.StatusCreated, result)
}

// HandleWebhook verifies a Stripe webhook signature and dispatches the event.
// MUST be mounted with raw body access — before any JSON-decoding middleware —
// because Stripe signature verification operates on the byte payload.
func (h *StripeHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.enabled() {
		WriteError(w, r, domain.ServiceUnavailable("Stripe webhooks are not configured"))
		return
	}

	const maxBody = 1 << 20 // 1 MiB — Stripe payloads are well under this
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		slog.Warn("stripe webhook read body failed", "error", err)
		WriteError(w, r, domain.BadRequest("failed to read request body"))
		return
	}

	signature := r.Header.Get("Stripe-Signature")
	if signature == "" {
		slog.Warn("stripe webhook missing signature")
		WriteError(w, r, domain.Unauthorized("missing Stripe-Signature header"))
		return
	}

	event, err := h.client.ConstructEvent(body, signature)
	if err != nil {
		slog.Warn("stripe webhook signature verification failed", "error", err)
		WriteError(w, r, domain.Unauthorized("invalid Stripe webhook signature"))
		return
	}

	if appErr := h.svc.HandleEvent(r.Context(), event, body); appErr != nil {
		slog.Warn("stripe webhook handling failed", "event_id", event.ID, "type", event.Type, "error", appErr.Message)
		WriteError(w, r, appErr)
		return
	}

	w.WriteHeader(http.StatusOK)
}
