package dexpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// WebhookEvent represents a parsed DEXPAY webhook payload.
type WebhookEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// Event type constants.
const (
	EventCheckoutInitiated = "checkout.initiated"
	EventCheckoutCompleted = "checkout.completed"
	EventCheckoutFailed    = "checkout.failed"
	EventCheckoutCancelled = "checkout.cancelled"
	EventCheckoutRefunded  = "checkout.refunded"
)

// CheckoutWebhookData captures the fields we use from flat DEXPAY checkout event payloads.
type CheckoutWebhookData struct {
	Reference         string `json:"reference"`
	Amount            int    `json:"amount"`
	Currency          string `json:"currency"`
	Status            string `json:"status"`
	PaymentURL        string `json:"payment_url"`
	SandboxPaymentURL string `json:"sandbox_payment_url"`
}

// VerifySignature verifies the HMAC-SHA256 signature of a webhook payload.
func VerifySignature(payload []byte, signature, secret string) error {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("invalid webhook signature")
	}
	return nil
}
