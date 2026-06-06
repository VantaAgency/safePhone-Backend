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

func TestPlanDeviceCoverageAndCaps(t *testing.T) {
	// "Plus"-like plan: 2 smartphones, 1 tablet, 1 console; no computer/TV.
	plan := &Plan{MaxSmartphones: 2, MaxTablets: 1, MaxGameConsoles: 1}
	cases := []struct {
		t       DeviceType
		allowed bool
		max     int
	}{
		{DeviceTypeSmartphone, true, 2},
		{DeviceTypeTablet, true, 1},
		{DeviceTypeGameConsole, true, 1},
		{DeviceTypeComputer, false, 0},
		{DeviceTypeTV, false, 0},
	}
	for _, c := range cases {
		if got := PlanAllowsDeviceType(plan, c.t); got != c.allowed {
			t.Errorf("PlanAllowsDeviceType(%s) = %v, want %v", c.t, got, c.allowed)
		}
		if got := plan.MaxForDeviceType(c.t); got != c.max {
			t.Errorf("MaxForDeviceType(%s) = %d, want %d", c.t, got, c.max)
		}
	}
}

func TestRequiredVerificationPhotos(t *testing.T) {
	cases := map[DeviceType]int{
		DeviceTypeSmartphone:  2,
		DeviceTypeTablet:      2,
		DeviceTypeTV:          1,
		DeviceTypeComputer:    1,
		DeviceTypeGameConsole: 1,
	}
	for dt, want := range cases {
		if got := RequiredVerificationPhotos(dt); got != want {
			t.Errorf("RequiredVerificationPhotos(%s) = %d, want %d", dt, got, want)
		}
	}
}

func TestValidateDeviceInputBrandOptionalForNonPhones(t *testing.T) {
	if f := ValidateDeviceInput(nil, DeviceTypeGameConsole, "", "PlayStation 5", "", DeviceMetadata{}); len(f) != 0 {
		t.Errorf("console with a model and no brand should be valid, got %v", f)
	}
	if f := ValidateDeviceInput(nil, DeviceTypeGameConsole, "", "", "", DeviceMetadata{}); f["model"] == "" {
		t.Errorf("console without a model should fail on model, got %v", f)
	}
	if f := ValidateDeviceInput(nil, DeviceTypeSmartphone, "", "iPhone 15", "", DeviceMetadata{}); f["brand"] == "" {
		t.Errorf("smartphone without a brand should still fail on brand, got %v", f)
	}
}
