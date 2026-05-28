package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// DeviceService handles device business logic.
type DeviceService struct {
	repo    domain.DeviceRepository
	subRepo domain.SubscriptionRepository
}

// NewDeviceService creates a new device service.
func NewDeviceService(repo domain.DeviceRepository, subRepo domain.SubscriptionRepository) *DeviceService {
	return &DeviceService{repo: repo, subRepo: subRepo}
}

// Create registers a new device for the authenticated user.
func (s *DeviceService) Create(ctx context.Context, ac *auth.AuthContext, deviceType domain.DeviceType, brand, model, imei string, metadata domain.DeviceMetadata) (*domain.Device, *domain.AppError) {
	deviceType = domain.NormalizeDeviceType(string(deviceType))
	metadata = metadata.Normalize()

	if fields := domain.ValidateDeviceInput(nil, deviceType, brand, model, imei, metadata); len(fields) > 0 {
		return nil, domain.ValidationFailed("validation failed", fields)
	}

	// Only check IMEI uniqueness when one is provided; IMEI can be added later via Update.
	if imei != "" {
		existing, err := s.repo.GetByIMEI(ctx, imei)
		if err != nil {
			return nil, domain.InternalError(err)
		}
		if existing != nil {
			return nil, domain.Conflict("a device with this IMEI is already registered")
		}
	}

	device := &domain.Device{
		OrgID:      ac.OrgID,
		UserID:     ac.UserID,
		DeviceType: deviceType,
		Brand:      brand,
		Model:      model,
		Metadata:   metadata,
		IMEI:       imei,
		Status:     domain.DeviceStatusPending,
	}

	if err := s.repo.Create(ctx, device); err != nil {
		return nil, domain.InternalError(err)
	}

	return device, nil
}

// List returns devices for the authenticated user.
func (s *DeviceService) List(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.Device, *domain.AppError) {
	devices, err := s.repo.ListByOrgAndUser(ctx, ac.OrgID, ac.UserID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return devices, nil
}

// Get returns a single device, verifying ownership.
func (s *DeviceService) Get(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*domain.Device, *domain.AppError) {
	device, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if device == nil {
		return nil, domain.NotFound("device")
	}
	if appErr := ac.EnsureOwnership(device.OrgID, device.UserID, "device"); appErr != nil {
		return nil, appErr
	}
	return device, nil
}

// Update modifies a device, verifying ownership.
func (s *DeviceService) Update(ctx context.Context, ac *auth.AuthContext, id uuid.UUID, deviceType domain.DeviceType, brand, model, imei string, metadata *domain.DeviceMetadata) (*domain.Device, *domain.AppError) {
	device, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if device == nil {
		return nil, domain.NotFound("device")
	}
	if appErr := ac.EnsureOwnership(device.OrgID, device.UserID, "device"); appErr != nil {
		return nil, appErr
	}

	if deviceType != "" {
		device.DeviceType = domain.NormalizeDeviceType(string(deviceType))
	}
	device.Brand = brand
	device.Model = model
	if metadata != nil {
		device.Metadata = metadata.Normalize()
	}

	if fields := domain.ValidateDeviceInput(nil, device.DeviceType, device.Brand, device.Model, imei, device.Metadata); len(fields) > 0 {
		return nil, domain.ValidationFailed("validation failed", fields)
	}

	if imei != "" {
		// Ensure no other device already holds this IMEI.
		existing, err := s.repo.GetByIMEI(ctx, imei)
		if err != nil {
			return nil, domain.InternalError(err)
		}
		if existing != nil && existing.ID != id {
			return nil, domain.Conflict("a device with this IMEI is already registered")
		}
		device.IMEI = imei
	}

	if nextStatus, err := s.resolveDeviceStatus(ctx, device); err != nil {
		return nil, domain.InternalError(err)
	} else {
		device.Status = nextStatus
	}

	if err := s.repo.Update(ctx, device); err != nil {
		return nil, domain.InternalError(err)
	}

	return device, nil
}

// Delete soft-deletes a device, verifying ownership.
func (s *DeviceService) Delete(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) *domain.AppError {
	device, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.InternalError(err)
	}
	if device == nil {
		return domain.NotFound("device")
	}
	if appErr := ac.EnsureOwnership(device.OrgID, device.UserID, "device"); appErr != nil {
		return appErr
	}

	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return domain.InternalError(err)
	}

	return nil
}

func (s *DeviceService) resolveDeviceStatus(ctx context.Context, device *domain.Device) (domain.DeviceStatus, error) {
	if device == nil {
		return domain.DeviceStatusPending, nil
	}

	switch device.Status {
	case domain.DeviceStatusExpired, domain.DeviceStatusSuspended:
		return device.Status, nil
	}

	if s.subRepo == nil {
		if !device.RequiresIMEI() || device.IMEI != "" {
			return device.Status, nil
		}
		return domain.DeviceStatusPending, nil
	}

	sub, err := s.subRepo.GetByDeviceID(ctx, device.ID)
	if err != nil {
		return domain.DeviceStatusPending, err
	}
	if sub != nil && sub.Status == domain.SubscriptionStatusActive && (!device.RequiresIMEI() || device.IMEI != "") {
		return domain.DeviceStatusActive, nil
	}

	return domain.DeviceStatusPending, nil
}
