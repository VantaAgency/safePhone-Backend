package service

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/database"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// PartnerApplicationService handles partner application submissions and review.
type PartnerApplicationService struct {
	repo        domain.PartnerApplicationRepository
	userRepo    domain.UserRepository
	partnerRepo domain.PartnerRepository
	pool        *pgxpool.Pool
}

// NewPartnerApplicationService creates a new partner application service.
func NewPartnerApplicationService(
	repo domain.PartnerApplicationRepository,
	userRepo domain.UserRepository,
	partnerRepo domain.PartnerRepository,
	pool *pgxpool.Pool,
) *PartnerApplicationService {
	return &PartnerApplicationService{
		repo:        repo,
		userRepo:    userRepo,
		partnerRepo: partnerRepo,
		pool:        pool,
	}
}

// Submit saves a new partner application linked to the authenticated user.
func (s *PartnerApplicationService) Submit(ctx context.Context, ac *auth.AuthContext, storeName, fullName, phone, city, businessLocation string) (*domain.PartnerApplication, *domain.AppError) {
	existing, err := s.repo.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if existing != nil && existing.Status == string(domain.PartnerAppStatusPending) {
		return nil, domain.Conflict("you already have a pending partner application")
	}

	app := &domain.PartnerApplication{
		OrgID:            ac.OrgID,
		UserID:           ac.UserID,
		StoreName:        storeName,
		FullName:         fullName,
		Phone:            phone,
		City:             city,
		BusinessLocation: businessLocation,
	}
	if err := s.repo.Create(ctx, app); err != nil {
		return nil, domain.InternalError(err)
	}
	return app, nil
}

// GetMyApplication returns the authenticated user's latest partner application.
func (s *PartnerApplicationService) GetMyApplication(ctx context.Context, ac *auth.AuthContext) (*domain.PartnerApplication, *domain.AppError) {
	app, err := s.repo.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if app == nil {
		return nil, domain.NotFound("partner application")
	}
	return app, nil
}

// ListApplications returns all partner applications for the org (admin only).
func (s *PartnerApplicationService) ListApplications(ctx context.Context, ac *auth.AuthContext, status *string, limit, offset int) ([]domain.AdminPartnerApplication, *domain.AppError) {
	apps, err := s.repo.ListByOrg(ctx, ac.OrgID, status, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return apps, nil
}

// ReviewApplication approves or rejects a partner application.
func (s *PartnerApplicationService) ReviewApplication(ctx context.Context, ac *auth.AuthContext, appID uuid.UUID, decision string, rejectionReason *string, commissionPercentage *float64) (*domain.PartnerApplication, *domain.AppError) {
	app, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if app == nil {
		return nil, domain.NotFound("partner application")
	}
	if app.Status != string(domain.PartnerAppStatusPending) {
		return nil, domain.Conflict("application has already been reviewed")
	}

	now := time.Now()
	app.ReviewedBy = &ac.UserID
	app.ReviewedAt = &now

	switch decision {
	case "rejected":
		app.Status = string(domain.PartnerAppStatusRejected)
		app.RejectionReason = rejectionReason

		if err := s.repo.UpdateStatus(ctx, app); err != nil {
			return nil, domain.InternalError(err)
		}
		return app, nil

	case "approved":
		if !isValidCommissionPercentage(commissionPercentage) {
			return nil, domain.BadRequest("commission_percentage must be between 0 and 100 with up to 2 decimals")
		}
		app.Status = string(domain.PartnerAppStatusApproved)
		app.CommissionPercentage = commissionPercentage

		referralCode, err := generateUniqueReferralCode(ctx, s.partnerRepo)
		if err != nil {
			return nil, domain.InternalError(err)
		}

		if txErr := database.WithTransaction(ctx, s.pool, func(tx pgx.Tx) error {
			// 1. Update application status
			if _, err := tx.Exec(ctx, `
				UPDATE partner_applications
				SET status = $2, reviewed_by = $3, reviewed_at = $4
				WHERE id = $1
			`, app.ID, app.Status, app.ReviewedBy, app.ReviewedAt); err != nil {
				return err
			}

			// 2. Create the partner record with the admin-assigned commission percentage.
			if _, err := tx.Exec(ctx, `
				INSERT INTO partners (
					org_id, user_id, store_name, city, business_location, referral_code, commission_percentage, status
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
			`, app.OrgID, app.UserID, app.StoreName, app.City, app.BusinessLocation, referralCode, *commissionPercentage); err != nil {
				return err
			}

			// 3. Update user role to partner in users table
			if _, err := tx.Exec(ctx, `
				UPDATE users SET role = $2, updated_at = now()
				WHERE id = $1 AND deleted_at IS NULL
			`, app.UserID, string(auth.RolePartner)); err != nil {
				return err
			}

			// 4. Update Better Auth "user" table role
			if _, err := tx.Exec(ctx, `
				UPDATE "user" SET role = $2, "updatedAt" = now()
				WHERE id = (SELECT better_auth_id FROM users WHERE id = $1 AND deleted_at IS NULL)
			`, app.UserID, string(auth.RolePartner)); err != nil {
				return err
			}

			return nil
		}); txErr != nil {
			return nil, domain.InternalError(txErr)
		}

		return app, nil

	default:
		return nil, domain.BadRequest("decision must be 'approved' or 'rejected'")
	}
}

func isValidCommissionPercentage(percentage *float64) bool {
	if percentage == nil {
		return false
	}
	if math.IsNaN(*percentage) || math.IsInf(*percentage, 0) {
		return false
	}
	if *percentage <= 0 || *percentage > 100 {
		return false
	}
	return math.Abs((*percentage*100)-math.Round(*percentage*100)) < 1e-9
}
