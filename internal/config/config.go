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

	// Public callback URLs
	FrontendURL      string `env:"FRONTEND_URL" envDefault:"http://localhost:3000"`
	BackendPublicURL string `env:"BACKEND_PUBLIC_URL"`
}

// IsDevelopment returns true if the application is running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// DexpayEnabled returns true when DEXPAY credentials are configured.
func (c *Config) DexpayEnabled() bool {
	return c.DexpayAPIKey != "" && c.DexpayAPISecret != ""
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
	if err := cfg.normalizeAndValidateDexpay(); err != nil {
		return nil, fmt.Errorf("validating DEXPAY config: %w", err)
	}
	return cfg, nil
}
