package config

import (
	"fmt"
	"net/url"
	"strings"
)

const betterAuthJWKSPath = "/api/auth/jwks"

func (c *Config) normalizeAndValidateAuth() error {
	c.JWKSURL = normalizeJWKSURL(c.JWKSURL)
	c.JWTIssuer = strings.TrimSpace(c.JWTIssuer)

	if err := validateAbsoluteURL("JWKS_URL", c.JWKSURL); err != nil {
		return err
	}
	if c.JWTIssuer == "" {
		return fmt.Errorf("JWT_ISSUER must not be empty")
	}

	return nil
}

func normalizeJWKSURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return raw
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return raw
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch path {
	case "", "/":
		parsed.Path = betterAuthJWKSPath
	case "/api/auth":
		parsed.Path = betterAuthJWKSPath
	case "/jwks":
		parsed.Path = betterAuthJWKSPath
	}

	parsed.RawPath = ""
	return normalizeExternalURL(parsed.String())
}
