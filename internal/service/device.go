package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// DeviceService handles device business logic.
type DeviceService struct {
	repo       domain.DeviceRepository
	subRepo    domain.SubscriptionRepository
	planRepo   domain.PlanRepository
	subDevices domain.SubscriptionDevicesRepository
}

// NewDeviceService creates a new device service.
func NewDeviceService(
	repo domain.DeviceRepository,
	subRepo domain.SubscriptionRepository,
	planRepo domain.PlanRepository,
	subDevices domain.SubscriptionDevicesRepository,
) *DeviceService {
	return &DeviceService{repo: repo, subRepo: subRepo, planRepo: planRepo, subDevices: subDevices}
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

// AddToSubscription registers a device and attaches it to an existing
// subscription, enforcing the plan's coverage + per-type cap. No payment is
// taken — the subscription already covers up to its caps. The device starts
// pending and activates after admin verification.
func (s *DeviceService) AddToSubscription(
	ctx context.Context,
	ac *auth.AuthContext,
	subscriptionID uuid.UUID,
	deviceType domain.DeviceType,
	brand, model, imei string,
	metadata domain.DeviceMetadata,
	verificationPhotos []string,
	verificationVideo string,
) (*domain.Device, *domain.AppError) {
	sub, err := s.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if sub == nil {
		return nil, domain.NotFound("subscription")
	}
	if appErr := ac.EnsureOwnership(sub.OrgID, sub.UserID, "subscription"); appErr != nil {
		return nil, appErr
	}
	if sub.Status == domain.SubscriptionStatusCancelled || sub.Status == domain.SubscriptionStatusExpired {
		return nil, domain.BadRequest("subscription is not active")
	}

	plan, err := s.planRepo.GetByID(ctx, sub.PlanID)
	if err != nil {
		return nil, domain.InternalError(err)
	}

	deviceType = domain.NormalizeDeviceType(string(deviceType))
	metadata = metadata.Normalize()
	if fields := domain.ValidateDeviceInput(plan, deviceType, brand, model, imei, metadata); len(fields) > 0 {
		return nil, domain.ValidationFailed("validation failed", fields)
	}
	if appErr := validateVerificationMedia(deviceType, verificationPhotos, verificationVideo); appErr != nil {
		return nil, appErr
	}

	// Enforce the plan's per-type cap for this subscription.
	if plan != nil {
		counts, err := s.subDevices.CountByType(ctx, subscriptionID)
		if err != nil {
			return nil, domain.InternalError(err)
		}
		if counts[deviceType] >= plan.MaxForDeviceType(deviceType) {
			return nil, domain.BadRequest("device limit reached for this device type on this plan")
		}
	}

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

	if len(verificationPhotos) > 0 || verificationVideo != "" {
		if err := s.repo.SetVerificationMedia(ctx, device.ID, verificationPhotos, verificationVideo); err != nil {
			return nil, domain.InternalError(err)
		}
		device.VerificationPhotos = verificationPhotos
		device.VerificationStatus = domain.DeviceVerificationStatusPending
	}

	if err := s.subDevices.Attach(ctx, subscriptionID, device.ID); err != nil {
		return nil, domain.InternalError(err)
	}

	return device, nil
}

// ListSubscriptionDevices returns the devices attached to a subscription
// (verifying ownership) so the UI can compute remaining slots per type.
func (s *DeviceService) ListSubscriptionDevices(ctx context.Context, ac *auth.AuthContext, subscriptionID uuid.UUID) ([]domain.Device, *domain.AppError) {
	sub, err := s.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if sub == nil {
		return nil, domain.NotFound("subscription")
	}
	if appErr := ac.EnsureOwnership(sub.OrgID, sub.UserID, "subscription"); appErr != nil {
		return nil, appErr
	}
	devices, err := s.subDevices.ListBySubscription(ctx, subscriptionID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return devices, nil
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
