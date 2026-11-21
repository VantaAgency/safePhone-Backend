package config

import "testing"

func TestPaymentOrigins(t *testing.T) {
	for _, tc := range []struct {
		name, mode, shared, sn, us string
		wantSN, wantUS             string
		reject                     bool
	}{
		{"production separates payment returns", "production", "https://www.safephone.sn", "", "", "https://www.safephone.sn", "https://www.safephone.us", false},
		{"development uses the configured tunnel", "development", "https://example.test", "", "", "https://example.test", "https://example.test", false},
		{"reject the wrong market origin", "production", "https://example.test", "", "https://www.safephone.sn/", "", "", true},
		{"reject a path in a staging origin", "production", "https://example.test", "https://sn.example.test/other", "", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Environment: tc.mode, FrontendURL: tc.shared, SNFrontendURL: tc.sn, USFrontendURL: tc.us}
			err := cfg.normalizeAndValidatePaymentOrigins()
			if tc.reject {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil || cfg.SNFrontendURL != tc.wantSN || cfg.USFrontendURL != tc.wantUS {
				t.Fatalf("payment origins = (%q, %q), error = %v", cfg.SNFrontendURL, cfg.USFrontendURL, err)
			}
		})
	}
}
