package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/cherif-safephone/safephone-backend/internal/dexpay"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/service"
)

// WebhookHandler handles incoming webhook events from payment providers.
type WebhookHandler struct {
	paymentSvc   *service.PaymentService
	dexpaySecret string
	devMode      bool
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(paymentSvc *service.PaymentService, dexpaySecret string, devMode bool) *WebhookHandler {
	return &WebhookHandler{paymentSvc: paymentSvc, dexpaySecret: dexpaySecret, devMode: devMode}
}

// HandleDexpay processes incoming DEXPAY webhook events.
func (h *WebhookHandler) HandleDexpay(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		WriteError(w, r, domain.BadRequest("failed to read request body"))
		return
	}

	signature := r.Header.Get("x-webhook-signature")
	if signature == "" {
		signature = r.Header.Get("x-dexchange-signature")
	}

	slog.Debug("dexpay webhook received",
		"body_len", len(body),
		"signature_present", signature != "",
		"event_header", r.Header.Get("x-webhook-event"),
	)

	// Verify HMAC-SHA256 signature (skip in dev mode when no signature is present)
	if !h.devMode || signature != "" {
		if err := dexpay.VerifySignature(body, signature, h.dexpaySecret); err != nil {
			slog.Warn("webhook signature mismatch", "signature_present", signature != "", "body_len", len(body))
			WriteError(w, r, domain.Unauthorized("invalid webhook signature"))
			return
		}
	}

	// Parse event — DEXPAY sends flat payloads (all fields at top level, no nested "data")
	var event dexpay.WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		slog.Warn("webhook unmarshal failed", "error", err, "body_len", len(body))
		WriteError(w, r, domain.BadRequest("invalid webhook payload"))
		return
	}
	// Use the full body as Data since DEXPAY payloads are flat
	event.Data = body

	slog.Debug("webhook event parsed", "event_type", event.Event, "data_len", len(event.Data))

	// Process event
	if appErr := h.paymentSvc.HandleWebhookEvent(r.Context(), event, body); appErr != nil {
		slog.Warn("webhook processing failed", "event", event.Event, "error", appErr.Message)
		WriteError(w, r, appErr)
		return
	}

	w.WriteHeader(http.StatusOK)
}
