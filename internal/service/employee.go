package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

const employeeOverviewSectionLimit = 8

// EmployeeService coordinates operational tooling for employee users.
type EmployeeService struct {
	repo      domain.EmployeeRepository
	userRepo  domain.UserRepository
	subRepo   domain.SubscriptionRepository
	claimRepo domain.ClaimRepository
	repairSvc *RepairService
}

// NewEmployeeService creates a new employee service.
func NewEmployeeService(
	repo domain.EmployeeRepository,
	userRepo domain.UserRepository,
	subRepo domain.SubscriptionRepository,
	claimRepo domain.ClaimRepository,
	repairSvc *RepairService,
) *EmployeeService {
	return &EmployeeService{
		repo:      repo,
		userRepo:  userRepo,
		subRepo:   subRepo,
		claimRepo: claimRepo,
		repairSvc: repairSvc,
	}
}

// GetOverview returns the employee workspace overview payload.
func (s *EmployeeService) GetOverview(ctx context.Context, ac *auth.AuthContext) (*domain.EmployeeDashboardOverview, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	metrics, err := s.repo.GetOverviewMetrics(ctx, ac.OrgID)
	if err != nil {
		return nil, domain.InternalError(err)
	}

	paymentFollowUps, err := s.repo.ListPaymentFollowUps(ctx, ac.OrgID, "", employeeOverviewSectionLimit, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	paymentFollowUps = filterPaymentAttention(paymentFollowUps, employeeOverviewSectionLimit)

	claimItems, err := s.repo.ListClaims(ctx, ac.OrgID, nil, "", 32, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	pendingClaims := filterPendingClaims(claimItems, employeeOverviewSectionLimit)

	repairItems, err := s.repo.ListRepairs(ctx, ac.OrgID, nil, "", 32, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	activeRepairs := filterActiveRepairs(repairItems, employeeOverviewSectionLimit)

	allTasks, err := s.repo.ListTasks(ctx, ac.OrgID, 200, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	metrics.UrgentTasksCount = countUrgentTasks(allTasks)
	urgentTasks := selectUrgentTasks(allTasks, employeeOverviewSectionLimit)

	return &domain.EmployeeDashboardOverview{
		Metrics:          *metrics,
		PaymentFollowUps: paymentFollowUps,
		PendingClaims:    pendingClaims,
		ActiveRepairs:    activeRepairs,
		UrgentTasks:      urgentTasks,
	}, nil
}

// ListClients returns employee-facing client summaries.
func (s *EmployeeService) ListClients(ctx context.Context, ac *auth.AuthContext, search string, limit, offset int) ([]domain.EmployeeClientListItem, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	items, err := s.repo.ListClients(ctx, ac.OrgID, search, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

// GetClient returns a single client detail for employees.
func (s *EmployeeService) GetClient(ctx context.Context, ac *auth.AuthContext, userID uuid.UUID) (*domain.EmployeeClientDetail, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	item, err := s.repo.GetClient(ctx, ac.OrgID, userID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if item == nil {
		return nil, domain.NotFound("client")
	}
	return item, nil
}

// ListPaymentFollowUps returns payment and activation follow-up rows.
func (s *EmployeeService) ListPaymentFollowUps(ctx context.Context, ac *auth.AuthContext, search string, limit, offset int) ([]domain.EmployeePaymentFollowUpItem, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	items, err := s.repo.ListPaymentFollowUps(ctx, ac.OrgID, search, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

// ListClaims returns employee claim detail rows.
func (s *EmployeeService) ListClaims(ctx context.Context, ac *auth.AuthContext, status *string, search string, limit, offset int) ([]domain.EmployeeClaimDetail, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	items, err := s.repo.ListClaims(ctx, ac.OrgID, status, search, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

// GetClaim returns an employee claim detail.
func (s *EmployeeService) GetClaim(ctx context.Context, ac *auth.AuthContext, claimID uuid.UUID) (*domain.EmployeeClaimDetail, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	item, err := s.repo.GetClaim(ctx, ac.OrgID, claimID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if item == nil {
		return nil, domain.NotFound("claim")
	}
	return item, nil
}

// UpdateClaimStatus performs the employee claim triage step.
func (s *EmployeeService) UpdateClaimStatus(ctx context.Context, ac *auth.AuthContext, claimID uuid.UUID, status domain.ClaimStatus) (*domain.Claim, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	if status != domain.ClaimStatusReview {
		return nil, domain.BadRequest("employees can only move claims into review")
	}

	claim, err := s.claimRepo.GetByID(ctx, claimID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if claim == nil || claim.OrgID != ac.OrgID {
		return nil, domain.NotFound("claim")
	}
	if claim.Status != domain.ClaimStatusPending {
		return nil, domain.BadRequest("only pending claims can be moved into review")
	}

	now := time.Now()
	claim.Status = domain.ClaimStatusReview
	claim.ReviewedAt = &now
	if err := s.claimRepo.Update(ctx, claim); err != nil {
		return nil, domain.InternalError(err)
	}

	return claim, nil
}

// ListRepairs returns employee repair detail rows.
func (s *EmployeeService) ListRepairs(ctx context.Context, ac *auth.AuthContext, status *string, search string, limit, offset int) ([]domain.EmployeeRepairDetail, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	items, err := s.repo.ListRepairs(ctx, ac.OrgID, status, search, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

// GetRepair returns a single repair detail for employees.
func (s *EmployeeService) GetRepair(ctx context.Context, ac *auth.AuthContext, repairID uuid.UUID) (*domain.EmployeeRepairDetail, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	item, err := s.repo.GetRepair(ctx, ac.OrgID, repairID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if item == nil {
		return nil, domain.NotFound("repair request")
	}
	return item, nil
}

// UpdateRepairStatus updates a repair request using the existing transition rules.
func (s *EmployeeService) UpdateRepairStatus(ctx context.Context, ac *auth.AuthContext, repairID uuid.UUID, status string, scheduledDate, scheduledTime *string) (*domain.RepairBooking, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	return s.repairSvc.AdminUpdateStatus(ctx, ac, repairID, status, scheduledDate, scheduledTime)
}

// UpdateRepairAmount updates the repair quote amount.
func (s *EmployeeService) UpdateRepairAmount(ctx context.Context, ac *auth.AuthContext, repairID uuid.UUID, amountXOF int) (*domain.RepairBooking, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	return s.repairSvc.AdminUpdateAmount(ctx, ac, repairID, amountXOF)
}

// ListTasks returns the operational task queue.
func (s *EmployeeService) ListTasks(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.EmployeeTaskItem, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	items, err := s.repo.ListTasks(ctx, ac.OrgID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

// GetFollowUp returns the current follow-up record for an entity.
func (s *EmployeeService) GetFollowUp(ctx context.Context, ac *auth.AuthContext, entityType domain.OperationalEntityType, entityID uuid.UUID) (*domain.OperationalFollowUp, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	if appErr := s.ensureOperationalEntity(ctx, ac, entityType, entityID); appErr != nil {
		return nil, appErr
	}

	item, err := s.repo.GetFollowUp(ctx, ac.OrgID, entityType, entityID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return item, nil
}

// UpsertFollowUp creates or updates an operational follow-up.
func (s *EmployeeService) UpsertFollowUp(
	ctx context.Context,
	ac *auth.AuthContext,
	entityType domain.OperationalEntityType,
	entityID uuid.UUID,
	reason *string,
	status domain.FollowUpStatus,
	nextAction *string,
	lastContactAt *time.Time,
) (*domain.OperationalFollowUp, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	if appErr := s.ensureOperationalEntity(ctx, ac, entityType, entityID); appErr != nil {
		return nil, appErr
	}

	actorID := ac.UserID
	followUp := &domain.OperationalFollowUp{
		OrgID:         ac.OrgID,
		EntityType:    entityType,
		EntityID:      entityID,
		Reason:        trimOptionalString(reason),
		Status:        status,
		NextAction:    trimOptionalString(nextAction),
		LastContactAt: lastContactAt,
		CreatedBy:     &actorID,
		UpdatedBy:     &actorID,
	}
	if err := s.repo.UpsertFollowUp(ctx, followUp); err != nil {
		return nil, domain.InternalError(err)
	}

	return followUp, nil
}

// ListNotes returns internal operational notes for an entity.
func (s *EmployeeService) ListNotes(ctx context.Context, ac *auth.AuthContext, entityType domain.OperationalEntityType, entityID uuid.UUID, limit, offset int) ([]domain.OperationalNote, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	if appErr := s.ensureOperationalEntity(ctx, ac, entityType, entityID); appErr != nil {
		return nil, appErr
	}

	items, err := s.repo.ListNotes(ctx, ac.OrgID, entityType, entityID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

// CreateNote appends an internal operational note.
func (s *EmployeeService) CreateNote(ctx context.Context, ac *auth.AuthContext, entityType domain.OperationalEntityType, entityID uuid.UUID, body string) (*domain.OperationalNote, *domain.AppError) {
	if appErr := s.requireActiveEmployee(ctx, ac); appErr != nil {
		return nil, appErr
	}

	if appErr := s.ensureOperationalEntity(ctx, ac, entityType, entityID); appErr != nil {
		return nil, appErr
	}

	trimmedBody := strings.TrimSpace(body)
	if trimmedBody == "" {
		return nil, domain.BadRequest("note body is required")
	}

	note := &domain.OperationalNote{
		OrgID:      ac.OrgID,
		EntityType: entityType,
		EntityID:   entityID,
		Body:       trimmedBody,
		CreatedBy:  ac.UserID,
	}
	if err := s.repo.CreateNote(ctx, note); err != nil {
		return nil, domain.InternalError(err)
	}

	notes, err := s.repo.ListNotes(ctx, ac.OrgID, entityType, entityID, 1, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if len(notes) > 0 && notes[0].ID == note.ID {
		return &notes[0], nil
	}

	return note, nil
}

func (s *EmployeeService) ensureOperationalEntity(ctx context.Context, ac *auth.AuthContext, entityType domain.OperationalEntityType, entityID uuid.UUID) *domain.AppError {
	switch entityType {
	case domain.OperationalEntityTypeClient:
		user, err := s.userRepo.GetByID(ctx, entityID)
		if err != nil {
			return domain.InternalError(err)
		}
		if user == nil || user.OrgID != ac.OrgID || user.DeletedAt != nil || user.Role != string(auth.RoleMember) {
			return domain.NotFound("client")
		}
	case domain.OperationalEntityTypeSubscription:
		sub, err := s.subRepo.GetByID(ctx, entityID)
		if err != nil {
			return domain.InternalError(err)
		}
		if sub == nil || sub.OrgID != ac.OrgID {
			return domain.NotFound("subscription")
		}
	case domain.OperationalEntityTypeClaim:
		claim, err := s.claimRepo.GetByID(ctx, entityID)
		if err != nil {
			return domain.InternalError(err)
		}
		if claim == nil || claim.OrgID != ac.OrgID {
			return domain.NotFound("claim")
		}
	case domain.OperationalEntityTypeRepair:
		if _, appErr := s.repairSvc.AdminGet(ctx, ac, entityID); appErr != nil {
			return appErr
		}
	default:
		return domain.BadRequest("invalid entity type")
	}

	return nil
}

func (s *EmployeeService) requireActiveEmployee(ctx context.Context, ac *auth.AuthContext) *domain.AppError {
	profile, err := s.userRepo.GetEmployeeProfile(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return domain.InternalError(err)
	}
	if profile == nil {
		return domain.Forbidden("employee access is not configured")
	}
	if profile.Status != domain.EmployeeAccountStatusActive {
		return domain.Forbidden("employee workspace access is disabled")
	}
	return nil
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func filterPaymentAttention(items []domain.EmployeePaymentFollowUpItem, limit int) []domain.EmployeePaymentFollowUpItem {
	filtered := make([]domain.EmployeePaymentFollowUpItem, 0, minInt(limit, len(items)))
	for _, item := range items {
		if !item.RequiresAttention {
			continue
		}
		filtered = append(filtered, item)
		if len(filtered) == limit {
			break
		}
	}
	if filtered == nil {
		return []domain.EmployeePaymentFollowUpItem{}
	}
	return filtered
}

func filterPendingClaims(items []domain.EmployeeClaimDetail, limit int) []domain.EmployeeClaimDetail {
	filtered := make([]domain.EmployeeClaimDetail, 0, minInt(limit, len(items)))
	for _, item := range items {
		if item.Claim.Status != domain.ClaimStatusPending && item.Claim.Status != domain.ClaimStatusReview {
			continue
		}
		filtered = append(filtered, item)
		if len(filtered) == limit {
			break
		}
	}
	if filtered == nil {
		return []domain.EmployeeClaimDetail{}
	}
	return filtered
}

func filterActiveRepairs(items []domain.EmployeeRepairDetail, limit int) []domain.EmployeeRepairDetail {
	filtered := make([]domain.EmployeeRepairDetail, 0, minInt(limit, len(items)))
	for _, item := range items {
		switch item.Repair.Status {
		case domain.RepairStatusAccepted, domain.RepairStatusScheduled, domain.RepairStatusInProgress:
			filtered = append(filtered, item)
		}
		if len(filtered) == limit {
			break
		}
	}
	if filtered == nil {
		return []domain.EmployeeRepairDetail{}
	}
	return filtered
}

func countUrgentTasks(items []domain.EmployeeTaskItem) int {
	count := 0
	for _, item := range items {
		if item.Priority == "high" {
			count++
		}
	}
	return count
}

func selectUrgentTasks(items []domain.EmployeeTaskItem, limit int) []domain.EmployeeTaskItem {
	urgent := make([]domain.EmployeeTaskItem, 0, minInt(limit, len(items)))
	for _, item := range items {
		if item.Priority != "high" {
			continue
		}
		urgent = append(urgent, item)
		if len(urgent) == limit {
			return urgent
		}
	}

	if len(urgent) > 0 {
		return urgent
	}

	fallback := minInt(limit, len(items))
	if fallback == 0 {
		return []domain.EmployeeTaskItem{}
	}
	return items[:fallback]
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
