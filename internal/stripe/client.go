// Package stripe wraps the Stripe Go SDK for SafePhone's US market.
//
// stripe-go uses package-level globals for the API key, so this package is
// effectively a singleton. Call NewClient once during process startup.
package stripe

import (
	"context"
	"fmt"
	"strings"

	stripego "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Client wraps the Stripe SDK with the configuration SafePhone needs.
type Client struct {
	secretKey     string
	webhookSecret string
	successURL    string
	cancelURL     string
}

// Config configures the Stripe client.
type Config struct {
	SecretKey     string
	WebhookSecret string
	SuccessURL    string
	CancelURL     string
}

// NewClient initializes the Stripe SDK and returns a wrapper. Sets the
// package-level stripe.Key — only one Client instance should exist per
// process.
func NewClient(cfg Config) (*Client, error) {
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("stripe: secret key required")
	}
	if cfg.WebhookSecret == "" {
		return nil, fmt.Errorf("stripe: webhook secret required")
	}
	if cfg.SuccessURL == "" || cfg.CancelURL == "" {
		return nil, fmt.Errorf("stripe: success and cancel URLs required")
	}
	stripego.Key = cfg.SecretKey
	return &Client{
		secretKey:     cfg.SecretKey,
		webhookSecret: cfg.WebhookSecret,
		successURL:    cfg.SuccessURL,
		cancelURL:     cfg.CancelURL,
	}, nil
}

// IsTestMode reports whether the configured secret is a test key.
func (c *Client) IsTestMode() bool {
	return strings.HasPrefix(c.secretKey, "sk_test_")
}

// CreateCustomerParams identifies a Stripe customer being created for a user.
type CreateCustomerParams struct {
	Email  string
	Name   string
	UserID string
}

// CreateCustomer creates a new Stripe Customer and returns its ID.
func (c *Client) CreateCustomer(ctx context.Context, p CreateCustomerParams) (string, error) {
	if c == nil {
		return "", fmt.Errorf("stripe: client nil")
	}
	params := &stripego.CustomerParams{
		Email: stripego.String(p.Email),
	}
	if p.Name != "" {
		params.Name = stripego.String(p.Name)
	}
	if p.UserID != "" {
		params.AddMetadata("safephone_user_id", p.UserID)
	}
	params.Context = ctx
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe create customer: %w", err)
	}
	return cust.ID, nil
}

// CreateCheckoutSessionParams describes a SafePhone checkout request.
type CreateCheckoutSessionParams struct {
	CustomerID     string
	PriceID        string
	Metadata       map[string]string
	IdempotencyKey string
}

// CheckoutSession is what we return after creating a session.
type CheckoutSession struct {
	ID  string
	URL string
}

// CreateCheckoutSession creates a Stripe Checkout Session in subscription
// mode with the SafePhone metadata that webhook handlers later read.
func (c *Client) CreateCheckoutSession(
	ctx context.Context,
	p CreateCheckoutSessionParams,
) (*CheckoutSession, error) {
	if c == nil {
		return nil, fmt.Errorf("stripe: client nil")
	}
	if p.PriceID == "" {
		return nil, fmt.Errorf("stripe: price ID required")
	}
	if p.CustomerID == "" {
		return nil, fmt.Errorf("stripe: customer ID required")
	}

	params := &stripego.CheckoutSessionParams{
		Mode:       stripego.String(string(stripego.CheckoutSessionModeSubscription)),
		Customer:   stripego.String(p.CustomerID),
		SuccessURL: stripego.String(c.successURL),
		CancelURL:  stripego.String(c.cancelURL),
		LineItems: []*stripego.CheckoutSessionLineItemParams{{
			Price:    stripego.String(p.PriceID),
			Quantity: stripego.Int64(1),
		}},
		SubscriptionData: &stripego.CheckoutSessionSubscriptionDataParams{
			Metadata: p.Metadata,
		},
		AllowPromotionCodes: stripego.Bool(true),
	}
	for k, v := range p.Metadata {
		params.AddMetadata(k, v)
	}
	if p.IdempotencyKey != "" {
		params.SetIdempotencyKey(p.IdempotencyKey)
	}
	params.Context = ctx

	s, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe create checkout session: %w", err)
	}
	return &CheckoutSession{ID: s.ID, URL: s.URL}, nil
}

// ConstructEvent verifies the Stripe-Signature header and returns the parsed
// event. We pass IgnoreAPIVersionMismatch because the Stripe account API
// version (set in the dashboard) can drift ahead of stripe-go's pinned
// version — that's a release lag, not a security issue. HMAC signature
// verification and the 5-minute timestamp tolerance still run, so replay
// attacks are still blocked. We only deserialize fields that have been
// stable across versions (subscription ID, customer ID, period_start/end,
// metadata) so the mismatch is safe in practice.
func (c *Client) ConstructEvent(payload []byte, signatureHeader string) (stripego.Event, error) {
	if c == nil {
		return stripego.Event{}, fmt.Errorf("stripe: client nil")
	}
	return webhook.ConstructEventWithOptions(payload, signatureHeader, c.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
}
