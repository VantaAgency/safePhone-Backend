package config

import (
	"fmt"
	"strings"
)

// StripeConfigured returns true when any Stripe-related env var is present.
// Used to detect partial configuration (helpful error vs silent skip).
func (c *Config) StripeConfigured() bool {
	return c.StripeSecretKey != "" ||
		c.StripeWebhookSecret != "" ||
		c.StripePriceEssentiel != "" ||
		c.StripePriceEcranPlus != "" ||
		c.StripePricePlus != "" ||
		c.StripePricePremium != "" ||
		c.StripePriceTotal != ""
}

// StripeCredentialMode reports whether the configured key is test or live.
func (c *Config) StripeCredentialMode() string {
	switch {
	case strings.HasPrefix(c.StripeSecretKey, "sk_test_"):
		return "test"
	case strings.HasPrefix(c.StripeSecretKey, "sk_live_"):
		return "live"
	case c.StripeSecretKey == "":
		return "missing"
	default:
		return "unknown"
	}
}

func (c *Config) normalizeAndValidateStripe() error {
	c.StripeSecretKey = strings.TrimSpace(c.StripeSecretKey)
	c.StripeWebhookSecret = strings.TrimSpace(c.StripeWebhookSecret)
	c.StripePriceEssentiel = strings.TrimSpace(c.StripePriceEssentiel)
	c.StripePriceEcranPlus = strings.TrimSpace(c.StripePriceEcranPlus)
	c.StripePricePlus = strings.TrimSpace(c.StripePricePlus)
	c.StripePricePremium = strings.TrimSpace(c.StripePricePremium)
	c.StripePriceTotal = strings.TrimSpace(c.StripePriceTotal)

	if !c.StripeConfigured() {
		return nil
	}

	if c.StripeSecretKey == "" {
		return fmt.Errorf("STRIPE_SECRET_KEY is required when Stripe is enabled")
	}
	if c.StripeWebhookSecret == "" {
		return fmt.Errorf("STRIPE_WEBHOOK_SECRET is required when Stripe is enabled")
	}
	if c.StripeCredentialMode() == "unknown" {
		return fmt.Errorf("STRIPE_SECRET_KEY must start with sk_test_ or sk_live_")
	}
	if !strings.HasPrefix(c.StripeWebhookSecret, "whsec_") {
		return fmt.Errorf("STRIPE_WEBHOOK_SECRET must start with whsec_")
	}
	return nil
}
