package config

import (
	"strings"
	"testing"
)

func TestNormalizeAndValidateDexpay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         Config
		wantBaseURL string
		wantEnv     DexpayEnvironment
		wantErr     string
	}{
		{
			name:        "disabled when no dexpay settings are present",
			cfg:         Config{},
			wantBaseURL: "",
			wantEnv:     DexpayEnvironmentDisabled,
		},
		{
			name: "infer sandbox base url from test credentials",
			cfg: Config{
				DexpayAPIKey:     "pk_test_public",
				DexpayAPISecret:  "sk_test_secret",
				FrontendURL:      "https://safephone.example",
				BackendPublicURL: "https://api.safephone.example",
			},
			wantBaseURL: DexpaySandboxBaseURL,
			wantEnv:     DexpayEnvironmentSandbox,
		},
		{
			name: "normalize trailing slash on explicit base url",
			cfg: Config{
				DexpayAPIKey:     "pk_live_public",
				DexpayAPISecret:  "sk_live_secret",
				DexpayBaseURL:    "https://api.dexpay.africa/api/v1/",
				FrontendURL:      "https://safephone.example/",
				BackendPublicURL: "https://api.safephone.example/",
			},
			wantBaseURL: DexpayProductionBaseURL,
			wantEnv:     DexpayEnvironmentProduction,
		},
		{
			name: "reject incomplete credentials",
			cfg: Config{
				DexpayAPIKey:     "pk_test_public",
				FrontendURL:      "https://safephone.example",
				BackendPublicURL: "https://api.safephone.example",
			},
			wantErr: "requires both DEXPAY_API_KEY and DEXPAY_API_SECRET",
		},
		{
			name: "reject mismatched key and secret environments",
			cfg: Config{
				DexpayAPIKey:     "pk_test_public",
				DexpayAPISecret:  "sk_live_secret",
				FrontendURL:      "https://safephone.example",
				BackendPublicURL: "https://api.safephone.example",
			},
			wantErr: "must target the same environment",
		},
		{
			name: "reject test credentials against production base url",
			cfg: Config{
				DexpayAPIKey:     "pk_test_public",
				DexpayAPISecret:  "sk_test_secret",
				DexpayBaseURL:    DexpayProductionBaseURL,
				FrontendURL:      "https://safephone.example",
				BackendPublicURL: "https://api.safephone.example",
			},
			wantErr: "does not match sandbox credentials",
		},
		{
			name: "reject missing backend public url",
			cfg: Config{
				DexpayAPIKey:    "pk_test_public",
				DexpayAPISecret: "sk_test_secret",
				FrontendURL:     "https://safephone.example",
			},
			wantErr: "BACKEND_PUBLIC_URL is required",
		},
		{
			name: "reject invalid frontend url",
			cfg: Config{
				DexpayAPIKey:     "pk_test_public",
				DexpayAPISecret:  "sk_test_secret",
				FrontendURL:      "/relative",
				BackendPublicURL: "https://api.safephone.example",
			},
			wantErr: "FRONTEND_URL must be an absolute URL",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := tt.cfg
			err := cfg.normalizeAndValidateDexpay()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if cfg.DexpayBaseURL != tt.wantBaseURL {
				t.Fatalf("expected base URL %q, got %q", tt.wantBaseURL, cfg.DexpayBaseURL)
			}
			if env := cfg.DexpayEnvironment(); env != tt.wantEnv {
				t.Fatalf("expected environment %q, got %q", tt.wantEnv, env)
			}
		})
	}
}
