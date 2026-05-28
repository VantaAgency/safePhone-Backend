package domain

import "testing"

func TestIsValidIMEI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Publicly documented Luhn-valid IMEI examples used across the spec docs.
		{"valid iPhone test IMEI", "490154203237518", true},
		{"valid Samsung test IMEI", "356938035643809", true},

		// Wrong length.
		{"empty string", "", false},
		{"too short", "12345678901234", false},
		{"too long", "1234567890123456", false},

		// Non-digit characters.
		{"contains letter", "49015420323751a", false},
		{"contains space", "490154203237 18", false},
		{"contains dash", "49015420323751-", false},

		// 15 digits but failing checksum (these would pass the old `len=15,numeric` tag).
		{"all ones", "111111111111111", false},
		{"all nines", "999999999999999", false},
		{"sequential digits", "123456789012345", false},
		{"valid IMEI but one digit flipped", "490154203237519", false},

		// Whitespace trimming.
		{"valid with surrounding spaces", "  490154203237518  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidIMEI(tt.input); got != tt.want {
				t.Errorf("IsValidIMEI(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
