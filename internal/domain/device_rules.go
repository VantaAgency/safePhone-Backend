package domain

import "strings"

// IsValidIMEI checks that the IMEI is exactly 15 digits AND passes the Luhn
// checksum. The handler-level `len=15,numeric` tag catches obvious junk but
// accepts e.g. "111111111111111", which isn't a real IMEI. Real device IMEIs
// always satisfy Luhn — this catches typos and casual spoofing without any
// network call.
func IsValidIMEI(imei string) bool {
	imei = strings.TrimSpace(imei)
	if len(imei) != 15 {
		return false
	}
	var sum int
	for i := 0; i < 15; i++ {
		c := imei[14-i]
		if c < '0' || c > '9' {
			return false
		}
		digit := int(c - '0')
		// Luhn: double every second digit from the right (i.e. odd indexes
		// here since we're iterating right-to-left starting at i=0).
		if i%2 == 1 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}

// NormalizeDeviceType ensures empty values fall back to smartphone.
func NormalizeDeviceType(raw string) DeviceType {
	switch DeviceType(strings.TrimSpace(raw)) {
	case DeviceTypeTablet:
		return DeviceTypeTablet
	case DeviceTypeTV:
		return DeviceTypeTV
	case DeviceTypeComputer:
		return DeviceTypeComputer
	case DeviceTypeHomeElectronics:
		return DeviceTypeHomeElectronics
	default:
		return DeviceTypeSmartphone
	}
}

// Normalize trims device metadata inputs.
func (m DeviceMetadata) Normalize() DeviceMetadata {
	return DeviceMetadata{
		SerialNumber:     strings.TrimSpace(m.SerialNumber),
		ScreenSize:       strings.TrimSpace(m.ScreenSize),
		ComputerCategory: strings.TrimSpace(m.ComputerCategory),
		ProductSubtype:   strings.TrimSpace(m.ProductSubtype),
	}
}

// RequiresIMEI reports whether a device must provide an IMEI before activation.
func (d Device) RequiresIMEI() bool {
	return d.DeviceType == DeviceTypeSmartphone
}

// IsTotalPlan reports whether the plan unlocks multi-device coverage.
func IsTotalPlan(plan *Plan) bool {
	return plan != nil && strings.EqualFold(strings.TrimSpace(plan.Slug), TotalPlanSlug)
}

// PlanAllowsDeviceType enforces plan-specific device eligibility.
func PlanAllowsDeviceType(plan *Plan, deviceType DeviceType) bool {
	if deviceType == DeviceTypeSmartphone {
		return true
	}
	return IsTotalPlan(plan)
}

// ValidateDeviceInput validates device details and optional plan restrictions.
func ValidateDeviceInput(plan *Plan, deviceType DeviceType, brand, model, imei string, metadata DeviceMetadata) map[string]string {
	fields := map[string]string{}

	normalizedType := NormalizeDeviceType(string(deviceType))
	normalizedBrand := strings.TrimSpace(brand)
	normalizedModel := strings.TrimSpace(model)
	normalizedIMEI := strings.TrimSpace(imei)
	normalizedMetadata := metadata.Normalize()

	if normalizedBrand == "" {
		fields["brand"] = "brand is required"
	}
	if normalizedModel == "" {
		fields["model"] = "model is required"
	}

	if plan != nil && !PlanAllowsDeviceType(plan, normalizedType) {
		fields["device_type"] = "this plan only supports smartphone registration"
	}

	if normalizedType != DeviceTypeSmartphone && normalizedIMEI != "" {
		fields["imei"] = "IMEI is only supported for smartphones"
	}
	if normalizedIMEI != "" && !IsValidIMEI(normalizedIMEI) {
		fields["imei"] = "IMEI is not valid (must be 15 digits with a valid checksum)"
	}
	if normalizedType == DeviceTypeComputer && normalizedMetadata.ComputerCategory == "" {
		fields["metadata.computer_category"] = "computer category is required for computers"
	}
	if normalizedType == DeviceTypeHomeElectronics && normalizedMetadata.ProductSubtype == "" {
		fields["metadata.product_subtype"] = "product subtype is required for home electronics"
	}

	return fields
}
