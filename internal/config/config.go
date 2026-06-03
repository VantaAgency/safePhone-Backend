// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port        int    `env:"PORT" envDefault:"8080"`
	Environment string `env:"ENVIRONMENT" envDefault:"development"`

	// Database
	DatabaseURL string `env:"DATABASE_URL,required"`

	// Redis
	RedisURL string `env:"REDIS_URL,required"`

	// Auth
	JWKSURL   string `env:"JWKS_URL,required"`
	JWTIssuer string `env:"JWT_ISSUER" envDefault:"safephone"`

	// CORS
	CORSOrigins string `env:"CORS_ORIGINS" envDefault:"http://localhost:3000"`

	// Rate Limiting
	RateLimitGeneral int `env:"RATE_LIMIT_GENERAL" envDefault:"100"`
	RateLimitAuth    int `env:"RATE_LIMIT_AUTH" envDefault:"10"`

	// Timeouts
	DBQueryTimeout      time.Duration `env:"DB_QUERY_TIMEOUT" envDefault:"5s"`
	CacheTimeout        time.Duration `env:"CACHE_TIMEOUT" envDefault:"500ms"`
	ExternalHTTPTimeout time.Duration `env:"EXTERNAL_HTTP_TIMEOUT" envDefault:"10s"`

	// DEXPAY payment gateway
	DexpayAPIKey    string `env:"DEXPAY_API_KEY"`
	DexpayAPISecret string `env:"DEXPAY_API_SECRET"`
	DexpayBaseURL   string `env:"DEXPAY_BASE_URL"`

	// Stripe payment gateway (US market)
	StripeSecretKey         string `env:"STRIPE_SECRET_KEY"`
	StripeWebhookSecret     string `env:"STRIPE_WEBHOOK_SECRET"`
	StripePriceEssentiel    string `env:"STRIPE_PRICE_ESSENTIEL"`
	StripePriceEcranPlus    string `env:"STRIPE_PRICE_ECRAN_PLUS"`
	StripePricePlus         string `env:"STRIPE_PRICE_PLUS"`
	StripePricePremium      string `env:"STRIPE_PRICE_PREMIUM"`
	StripePriceTotal        string `env:"STRIPE_PRICE_TOTAL"`
	StripeSuccessPath       string `env:"STRIPE_SUCCESS_PATH" envDefault:"/us/checkout/success?session_id={CHECKOUT_SESSION_ID}"`
	StripeCancelPath        string `env:"STRIPE_CANCEL_PATH" envDefault:"/us/checkout/cancel"`

	// Public callback URLs
	FrontendURL      string `env:"FRONTEND_URL" envDefault:"http://localhost:3000"`
	BackendPublicURL string `env:"BACKEND_PUBLIC_URL"`

	// S3-compatible storage for commercial activity photos (Railway Buckets)
	S3Endpoint        string `env:"S3_ENDPOINT"`
	S3Region          string `env:"S3_REGION" envDefault:"auto"`
	S3Bucket          string `env:"S3_BUCKET"`
	S3AccessKeyID     string `env:"S3_ACCESS_KEY_ID"`
	S3SecretAccessKey string `env:"S3_SECRET_ACCESS_KEY"`
	S3ActivityPrefix     string `env:"S3_ACTIVITY_PREFIX" envDefault:"commercial-activity"`
	S3VerificationPrefix string `env:"S3_VERIFICATION_PREFIX" envDefault:"verification-uploads"`
	S3ForcePathStyle     bool   `env:"S3_FORCE_PATH_STYLE" envDefault:"false"`
}

// IsDevelopment returns true if the application is running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// DexpayEnabled returns true when DEXPAY credentials are configured.
func (c *Config) DexpayEnabled() bool {
	return c.DexpayAPIKey != "" && c.DexpayAPISecret != ""
}

// StripeEnabled returns true when Stripe credentials are configured.
func (c *Config) StripeEnabled() bool {
	return c.StripeSecretKey != "" && c.StripeWebhookSecret != ""
}

// StripePriceIDForPlan returns the configured Stripe price ID for the
// given US plan slug. Empty string means the plan isn't wired for Stripe.
func (c *Config) StripePriceIDForPlan(planSlug string) string {
	switch planSlug {
	case "us_essentiel":
		return c.StripePriceEssentiel
	case "us_ecran_plus":
		return c.StripePriceEcranPlus
	case "us_plus":
		return c.StripePricePlus
	case "us_premium":
		return c.StripePricePremium
	case "us_total":
		return c.StripePriceTotal
	default:
		return ""
	}
}

// S3Enabled returns true when S3-compatible object storage credentials are configured.
func (c *Config) S3Enabled() bool {
	return c.S3Endpoint != "" &&
		c.S3Bucket != "" &&
		c.S3AccessKeyID != "" &&
		c.S3SecretAccessKey != ""
}

// S3PartiallyConfigured returns true when only some required S3 variables are present.
func (c *Config) S3PartiallyConfigured() bool {
	hasAny := c.S3Endpoint != "" ||
		c.S3Bucket != "" ||
		c.S3AccessKeyID != "" ||
		c.S3SecretAccessKey != ""
	return hasAny && !c.S3Enabled()
}

// Load reads configuration from environment variables and returns a Config.
// It fails fast if required variables are missing.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := cfg.normalizeAndValidateAuth(); err != nil {
		return nil, fmt.Errorf("validating auth config: %w", err)
	}
	if err := cfg.normalizeAndValidateCORS(); err != nil {
		return nil, fmt.Errorf("validating CORS config: %w", err)
	}
	if err := cfg.normalizeAndValidateDexpay(); err != nil {
		return nil, fmt.Errorf("validating DEXPAY config: %w", err)
	}
	if err := cfg.normalizeAndValidateStripe(); err != nil {
		return nil, fmt.Errorf("validating Stripe config: %w", err)
	}
	return cfg, nil
}
