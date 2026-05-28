package config

import (
	"strings"
	"testing"
)

func TestNormalizeAndValidateCORS(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   string // substring of expected error; empty = expect no error
		wantClean string // expected CORSOrigins after normalization (only when no error)
	}{
		{
			name:      "single localhost origin",
			input:     "http://localhost:3000",
			wantClean: "http://localhost:3000",
		},
		{
			name:      "trailing slash is stripped",
			input:     "https://app.safephone.io/",
			wantClean: "https://app.safephone.io",
		},
		{
			name:      "csv with whitespace and empty entries",
			input:     " http://localhost:3000 , , https://staging.safephone.io ",
			wantClean: "http://localhost:3000,https://staging.safephone.io",
		},
		{name: "empty rejected", input: "", wantErr: "must list at least one"},
		{name: "wildcard rejected", input: "*", wantErr: "is not allowed"},
		{name: "wildcard among real origins rejected", input: "http://localhost:3000,*", wantErr: "is not allowed"},
		{name: "ftp scheme rejected", input: "ftp://files.safephone.io", wantErr: "must use http or https"},
		{name: "missing scheme rejected", input: "localhost:3000", wantErr: "must use http or https"},
		{name: "path-in-origin rejected", input: "https://app.safephone.io/dashboard", wantErr: "must not include a path"},
		{name: "missing host rejected", input: "https://", wantErr: "missing a host"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{CORSOrigins: tt.input}
			err := c.normalizeAndValidateCORS()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if c.CORSOrigins != tt.wantClean {
					t.Fatalf("expected CORSOrigins=%q, got %q", tt.wantClean, c.CORSOrigins)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
