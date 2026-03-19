package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// RepairService handles business logic for repair bookings.
type RepairService struct {
	repo domain.RepairRepository
}

// NewRepairService creates a new repair service.
func NewRepairService(repo domain.RepairRepository) *RepairService {
	return &RepairService{repo: repo}
}

// CreateBooking saves a new repair booking and returns it with the generated reference.
func (s *RepairService) CreateBooking(
	ctx context.Context,
	orgID *uuid.UUID,
	userID *uuid.UUID,
	deviceType, repairType, locationID, bookingDate, bookingTime, customerName, customerPhone string,
) (*domain.RepairBooking, *domain.AppError) {
	booking := &domain.RepairBooking{
		OrgID:         orgID,
		UserID:        userID,
		DeviceType:    deviceType,
		RepairType:    repairType,
		LocationID:    locationID,
		BookingDate:   bookingDate,
		BookingTime:   bookingTime,
		CustomerName:  customerName,
		CustomerPhone: customerPhone,
	}
	if err := s.repo.Create(ctx, booking); err != nil {
		return nil, domain.InternalError(err)
	}
	return booking, nil
}
