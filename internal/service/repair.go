package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// RepairService handles business logic for repair bookings.
type RepairService struct {
	repo     domain.RepairRepository
	userRepo domain.UserRepository
}

// NewRepairService creates a new repair service.
func NewRepairService(repo domain.RepairRepository, userRepo domain.UserRepository) *RepairService {
	return &RepairService{repo: repo, userRepo: userRepo}
}

// CreateBooking saves a new repair booking and returns it with the generated reference.
func (s *RepairService) CreateBooking(
	ctx context.Context,
	ac *auth.AuthContext,
	deviceBrand, deviceModel, repairType, serviceMode string,
	centerID *string,
	preferredDate, preferredTime, customerName, customerPhone string,
) (*domain.RepairBooking, *domain.AppError) {
	deviceBrand = strings.TrimSpace(deviceBrand)
	deviceModel = strings.TrimSpace(deviceModel)
	repairType = strings.TrimSpace(repairType)
	serviceMode = strings.TrimSpace(serviceMode)
	customerName = strings.TrimSpace(customerName)
	customerPhone = strings.TrimSpace(customerPhone)
	normalizedPhone := normalizePhone(customerPhone)

	if deviceBrand == "" || deviceModel == "" || repairType == "" || preferredDate == "" || preferredTime == "" || customerName == "" || normalizedPhone == "" {
		return nil, domain.BadRequest("missing required repair request fields")
	}
	if serviceMode != domain.RepairServiceModeCenter && serviceMode != domain.RepairServiceModeHome {
		return nil, domain.BadRequest("invalid service mode")
	}

	var normalizedCenterID *string
	if serviceMode == domain.RepairServiceModeCenter {
		if centerID == nil || strings.TrimSpace(*centerID) == "" {
			return nil, domain.BadRequest("center_id is required for center repairs")
		}
		value := strings.TrimSpace(*centerID)
		normalizedCenterID = &value
	}

	var orgID *uuid.UUID
	var userID *uuid.UUID
	var market domain.MarketCode
	requestSource := domain.RepairRequestSourcePublicVisitor
	if ac != nil {
		orgID = &ac.OrgID
		userID = &ac.UserID
		requestSource = domain.RepairRequestSourceSafePhoneUser
		// A repair inherits the requesting user's market; public/anonymous
		// requests leave it empty and the repository defaults to SN.
		if user, err := s.userRepo.GetByID(ctx, ac.UserID); err == nil && user != nil {
			market = user.Market
		}
	}

	booking := &domain.RepairBooking{
		OrgID:                   orgID,
		UserID:                  userID,
		Market:                  market,
		DeviceBrand:             deviceBrand,
		DeviceModel:             deviceModel,
		RepairType:              repairType,
		ServiceMode:             serviceMode,
		CenterID:                normalizedCenterID,
		PreferredDate:           preferredDate,
		PreferredTime:           preferredTime,
		CustomerName:            customerName,
		CustomerPhone:           customerPhone,
		CustomerPhoneNormalized: normalizedPhone,
		Status:                  domain.RepairStatusPending,
		RequestSource:           requestSource,
	}
	if err := s.repo.Create(ctx, booking); err != nil {
		return nil, domain.InternalError(err)
	}
	return booking, nil
}

// LookupByReference returns a sanitized repair request for public tracking.
func (s *RepairService) LookupByReference(ctx context.Context, reference, customerPhone string) (*domain.RepairBooking, *domain.AppError) {
	normalizedPhone := normalizePhone(customerPhone)
	if strings.TrimSpace(reference) == "" || normalizedPhone == "" {
		return nil, domain.BadRequest("reference and customer phone are required")
	}

	booking, err := s.repo.GetByReferenceAndPhone(ctx, strings.ToUpper(strings.TrimSpace(reference)), normalizedPhone)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if booking == nil {
		return nil, domain.NotFound("repair request")
	}
	return booking, nil
}

// ListMine returns repair requests linked to the authenticated user.
func (s *RepairService) ListMine(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.RepairBooking, *domain.AppError) {
	bookings, err := s.repo.ListByOrgAndUser(ctx, ac.OrgID, ac.UserID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return bookings, nil
}

// AdminList returns org repair requests for admin users.
func (s *RepairService) AdminList(ctx context.Context, ac *auth.AuthContext, status *string, search string, limit, offset int) ([]domain.RepairBooking, *domain.AppError) {
	bookings, err := s.repo.ListByOrg(ctx, ac.OrgID, status, search, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return bookings, nil
}

// AdminGet returns a single repair request for admin users.
func (s *RepairService) AdminGet(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*domain.RepairBooking, *domain.AppError) {
	booking, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if booking == nil || booking.OrgID == nil || *booking.OrgID != ac.OrgID {
		return nil, domain.NotFound("repair request")
	}
	return booking, nil
}

// AdminAccept moves a repair request from pending to accepted.
func (s *RepairService) AdminAccept(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*domain.RepairBooking, *domain.AppError) {
	return s.transitionStatus(ctx, ac, id, domain.RepairStatusAccepted, nil, nil)
}

// AdminReject moves a repair request from pending to rejected.
func (s *RepairService) AdminReject(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*domain.RepairBooking, *domain.AppError) {
	return s.transitionStatus(ctx, ac, id, domain.RepairStatusRejected, nil, nil)
}

// AdminUpdateStatus updates the repair request lifecycle status.
func (s *RepairService) AdminUpdateStatus(ctx context.Context, ac *auth.AuthContext, id uuid.UUID, status string, scheduledDate, scheduledTime *string) (*domain.RepairBooking, *domain.AppError) {
	return s.transitionStatus(ctx, ac, id, status, scheduledDate, scheduledTime)
}

// AdminUpdateAmount updates the quoted repair amount independently of status.
func (s *RepairService) AdminUpdateAmount(ctx context.Context, ac *auth.AuthContext, id uuid.UUID, amountXOF int) (*domain.RepairBooking, *domain.AppError) {
	if amountXOF < 0 {
		return nil, domain.BadRequest("repair amount must be positive")
	}

	booking, appErr := s.AdminGet(ctx, ac, id)
	if appErr != nil {
		return nil, appErr
	}

	booking.RepairAmountMinor = &amountXOF
	if err := s.repo.Update(ctx, booking); err != nil {
		return nil, domain.InternalError(err)
	}
	return booking, nil
}

func (s *RepairService) transitionStatus(ctx context.Context, ac *auth.AuthContext, id uuid.UUID, nextStatus string, scheduledDate, scheduledTime *string) (*domain.RepairBooking, *domain.AppError) {
	booking, appErr := s.AdminGet(ctx, ac, id)
	if appErr != nil {
		return nil, appErr
	}

	if !isAllowedRepairTransition(booking.Status, nextStatus) {
		return nil, domain.BadRequest("invalid repair status transition")
	}

	if nextStatus == domain.RepairStatusScheduled {
		if scheduledDate == nil || strings.TrimSpace(*scheduledDate) == "" || scheduledTime == nil || strings.TrimSpace(*scheduledTime) == "" {
			return nil, domain.BadRequest("scheduled date and time are required")
		}
		date := strings.TrimSpace(*scheduledDate)
		time := strings.TrimSpace(*scheduledTime)
		booking.ScheduledDate = &date
		booking.ScheduledTime = &time
	}

	booking.Status = nextStatus
	if err := s.repo.Update(ctx, booking); err != nil {
		return nil, domain.InternalError(err)
	}
	return booking, nil
}

func normalizePhone(raw string) string {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}

func isAllowedRepairTransition(current, next string) bool {
	if current == next {
		return true
	}

	transitions := map[string]map[string]bool{
		domain.RepairStatusPending: {
			domain.RepairStatusAccepted:  true,
			domain.RepairStatusRejected:  true,
			domain.RepairStatusCancelled: true,
		},
		domain.RepairStatusAccepted: {
			domain.RepairStatusScheduled:  true,
			domain.RepairStatusInProgress: true,
			domain.RepairStatusCancelled:  true,
		},
		domain.RepairStatusScheduled: {
			domain.RepairStatusInProgress: true,
			domain.RepairStatusCompleted:  true,
			domain.RepairStatusCancelled:  true,
		},
		domain.RepairStatusInProgress: {
			domain.RepairStatusCompleted: true,
			domain.RepairStatusCancelled: true,
		},
	}

	return transitions[current][next]
}
