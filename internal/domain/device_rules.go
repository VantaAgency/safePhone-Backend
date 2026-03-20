package domain

import "strings"

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
	if normalizedType == DeviceTypeComputer && normalizedMetadata.ComputerCategory == "" {
		fields["metadata.computer_category"] = "computer category is required for computers"
	}
	if normalizedType == DeviceTypeHomeElectronics && normalizedMetadata.ProductSubtype == "" {
		fields["metadata.product_subtype"] = "product subtype is required for home electronics"
	}

	return fields
}
