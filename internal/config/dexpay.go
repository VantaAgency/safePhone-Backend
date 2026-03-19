package config

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	DexpaySandboxBaseURL    = "https://api-sandbox.dexpay.africa/api/v1"
	DexpayProductionBaseURL = "https://api.dexpay.africa/api/v1"
)

// DexpayEnvironment identifies the configured DEXPAY environment.
type DexpayEnvironment string

const (
	DexpayEnvironmentDisabled   DexpayEnvironment = "disabled"
	DexpayEnvironmentSandbox    DexpayEnvironment = "sandbox"
	DexpayEnvironmentProduction DexpayEnvironment = "production"
	DexpayEnvironmentUnknown    DexpayEnvironment = "unknown"
)

// DexpayConfigured returns true when any DEXPAY setting is present.
func (c *Config) DexpayConfigured() bool {
	return c.DexpayAPIKey != "" || c.DexpayAPISecret != "" || c.DexpayBaseURL != ""
}

// DexpayEnvironment returns the resolved DEXPAY environment.
func (c *Config) DexpayEnvironment() DexpayEnvironment {
	if !c.DexpayEnabled() {
		return DexpayEnvironmentDisabled
	}

	if env := inferDexpayEnvironmentFromBaseURL(c.DexpayBaseURL); env != DexpayEnvironmentUnknown {
		return env
	}
	if env := inferDexpayEnvironmentFromCredential(c.DexpayAPIKey); env != DexpayEnvironmentUnknown {
		return env
	}
	if env := inferDexpayEnvironmentFromCredential(c.DexpayAPISecret); env != DexpayEnvironmentUnknown {
		return env
	}

	return DexpayEnvironmentUnknown
}

// DexpayCredentialMode returns the configured key mode for logging.
func (c *Config) DexpayCredentialMode() string {
	switch inferDexpayEnvironmentFromCredential(c.DexpayAPIKey) {
	case DexpayEnvironmentSandbox:
		return "test"
	case DexpayEnvironmentProduction:
		return "live"
	default:
		return "unknown"
	}
}

func (c *Config) normalizeAndValidateDexpay() error {
	c.DexpayAPIKey = strings.TrimSpace(c.DexpayAPIKey)
	c.DexpayAPISecret = strings.TrimSpace(c.DexpayAPISecret)
	c.DexpayBaseURL = normalizeDexpayBaseURL(c.DexpayBaseURL)
	c.FrontendURL = normalizeExternalURL(c.FrontendURL)
	c.BackendPublicURL = normalizeExternalURL(c.BackendPublicURL)

	if !c.DexpayConfigured() {
		c.DexpayBaseURL = ""
		return nil
	}

	if c.DexpayAPIKey == "" || c.DexpayAPISecret == "" {
		return fmt.Errorf("DEXPAY requires both DEXPAY_API_KEY and DEXPAY_API_SECRET when the integration is enabled")
	}

	keyEnv := inferDexpayEnvironmentFromCredential(c.DexpayAPIKey)
	if keyEnv == DexpayEnvironmentUnknown {
		return fmt.Errorf("DEXPAY_API_KEY must start with pk_test_ or pk_live_")
	}

	secretEnv := inferDexpayEnvironmentFromCredential(c.DexpayAPISecret)
	if secretEnv == DexpayEnvironmentUnknown {
		return fmt.Errorf("DEXPAY_API_SECRET must start with sk_test_ or sk_live_")
	}

	if keyEnv != secretEnv {
		return fmt.Errorf("DEXPAY_API_KEY and DEXPAY_API_SECRET must target the same environment")
	}

	if c.DexpayBaseURL == "" {
		c.DexpayBaseURL = dexpayBaseURLForEnvironment(keyEnv)
	}

	baseEnv := inferDexpayEnvironmentFromBaseURL(c.DexpayBaseURL)
	if baseEnv == DexpayEnvironmentUnknown {
		return fmt.Errorf("DEXPAY_BASE_URL must be %q or %q", DexpaySandboxBaseURL, DexpayProductionBaseURL)
	}

	if baseEnv != keyEnv {
		return fmt.Errorf("DEXPAY_BASE_URL %q does not match %s credentials", c.DexpayBaseURL, keyEnv)
	}
	if c.BackendPublicURL == "" {
		return fmt.Errorf("BACKEND_PUBLIC_URL is required when DEXPAY is enabled")
	}
	if err := validateAbsoluteURL("BACKEND_PUBLIC_URL", c.BackendPublicURL); err != nil {
		return err
	}
	if c.FrontendURL == "" {
		return fmt.Errorf("FRONTEND_URL is required when DEXPAY is enabled")
	}
	if err := validateAbsoluteURL("FRONTEND_URL", c.FrontendURL); err != nil {
		return err
	}

	return nil
}

func normalizeDexpayBaseURL(baseURL string) string {
	return normalizeExternalURL(baseURL)
}

func dexpayBaseURLForEnvironment(env DexpayEnvironment) string {
	switch env {
	case DexpayEnvironmentSandbox:
		return DexpaySandboxBaseURL
	case DexpayEnvironmentProduction:
		return DexpayProductionBaseURL
	default:
		return ""
	}
}

func inferDexpayEnvironmentFromBaseURL(baseURL string) DexpayEnvironment {
	switch normalizeDexpayBaseURL(baseURL) {
	case DexpaySandboxBaseURL:
		return DexpayEnvironmentSandbox
	case DexpayProductionBaseURL:
		return DexpayEnvironmentProduction
	default:
		return DexpayEnvironmentUnknown
	}
}

func inferDexpayEnvironmentFromCredential(value string) DexpayEnvironment {
	switch {
	case strings.HasPrefix(value, "pk_test_"), strings.HasPrefix(value, "sk_test_"):
		return DexpayEnvironmentSandbox
	case strings.HasPrefix(value, "pk_live_"), strings.HasPrefix(value, "sk_live_"):
		return DexpayEnvironmentProduction
	default:
		return DexpayEnvironmentUnknown
	}
}

func normalizeExternalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	return raw
}

func validateAbsoluteURL(name, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", name)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", name)
	}
	return nil
}
