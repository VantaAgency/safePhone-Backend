package config

import "testing"

func TestNormalizeJWKSURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "origin only resolves to better auth jwks endpoint",
			in:   "https://safephone.example",
			want: "https://safephone.example/api/auth/jwks",
		},
		{
			name: "root path resolves to better auth jwks endpoint",
			in:   "https://safephone.example/",
			want: "https://safephone.example/api/auth/jwks",
		},
		{
			name: "api auth base resolves to jwks endpoint",
			in:   "https://safephone.example/api/auth",
			want: "https://safephone.example/api/auth/jwks",
		},
		{
			name: "legacy jwks path resolves to better auth auth route",
			in:   "https://safephone.example/jwks",
			want: "https://safephone.example/api/auth/jwks",
		},
		{
			name: "explicit jwks endpoint remains unchanged",
			in:   "https://safephone.example/api/auth/jwks",
			want: "https://safephone.example/api/auth/jwks",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeJWKSURL(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNormalizeAndValidateAuth(t *testing.T) {
	t.Parallel()

	cfg := Config{
		JWKSURL:   "https://safephone.example",
		JWTIssuer: "safephone",
	}

	if err := cfg.normalizeAndValidateAuth(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.JWKSURL != "https://safephone.example/api/auth/jwks" {
		t.Fatalf("expected normalized JWKS URL, got %q", cfg.JWKSURL)
	}
}
