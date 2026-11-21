package config

import (
	"fmt"
	"net/url"
	"strings"
)

func (c *Config) normalizeAndValidatePaymentOrigins() error {
	c.SNFrontendURL = normalizeExternalURL(c.SNFrontendURL)
	c.USFrontendURL = normalizeExternalURL(c.USFrontendURL)
	if c.SNFrontendURL == "" {
		c.SNFrontendURL = "https://www.safephone.sn"
		if c.IsDevelopment() {
			c.SNFrontendURL = c.FrontendURL
		}
	}
	if c.USFrontendURL == "" {
		c.USFrontendURL = "https://www.safephone.us"
		if c.IsDevelopment() {
			c.USFrontendURL = c.FrontendURL
		}
	}

	for _, origin := range []struct{ name, value, otherHost string }{
		{"SN_FRONTEND_URL", c.SNFrontendURL, "safephone.us"},
		{"US_FRONTEND_URL", c.USFrontendURL, "safephone.sn"},
	} {
		if err := validateAbsoluteURL(origin.name, origin.value); err != nil {
			return err
		}
		parsed, _ := url.Parse(origin.value)
		host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
		if parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
			host == origin.otherHost || host == "www."+origin.otherHost {
			return fmt.Errorf("%s must be the origin for its own market", origin.name)
		}
	}
	return nil
}
