package config

import (
	"fmt"
	"net/url"
	"strings"
)

// AllowedOrigins returns CORS_ORIGINS as a clean slice of validated absolute
// URLs. The chi cors middleware accepts a list — passing the raw string would
// silently drop typos and would happily accept "*" or empty entries.
func (c *Config) AllowedOrigins() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, 4)
	for _, raw := range strings.Split(c.CORSOrigins, ",") {
		o := strings.TrimSpace(raw)
		if o == "" {
			continue
		}
		out = append(out, o)
	}
	return out
}

// normalizeAndValidateCORS rejects dangerous CORS configurations at boot. We
// explicitly refuse "*" — even though chi-cors supports it, a wildcard
// effectively disables the protection the header is there to provide.
func (c *Config) normalizeAndValidateCORS() error {
	origins := c.AllowedOrigins()
	if len(origins) == 0 {
		return fmt.Errorf("CORS_ORIGINS must list at least one allowed origin")
	}
	clean := make([]string, 0, len(origins))
	for _, o := range origins {
		if o == "*" {
			return fmt.Errorf("CORS_ORIGINS=\"*\" is not allowed — list each trusted origin explicitly")
		}
		parsed, err := url.Parse(o)
		if err != nil || parsed == nil {
			return fmt.Errorf("CORS_ORIGINS entry %q is not a valid URL", o)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("CORS_ORIGINS entry %q must use http or https (got scheme %q)", o, parsed.Scheme)
		}
		if parsed.Host == "" {
			return fmt.Errorf("CORS_ORIGINS entry %q is missing a host", o)
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return fmt.Errorf("CORS_ORIGINS entry %q must not include a path", o)
		}
		// Strip trailing slash so the chi-cors comparison matches the
		// browser-sent Origin header (which never has one).
		normalized := strings.TrimRight(o, "/")
		clean = append(clean, normalized)
	}
	c.CORSOrigins = strings.Join(clean, ",")
	return nil
}
