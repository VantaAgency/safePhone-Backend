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

const defaultCommercialPartnerCommissionPercentage = 10.0

// PartnerApplicationService handles partner application submissions and review.
type PartnerApplicationService struct {
	repo           domain.PartnerApplicationRepository
	userRepo       domain.UserRepository
	partnerRepo    domain.PartnerRepository
	commercialRepo domain.CommercialRepository
	pool           *pgxpool.Pool
}

// NewPartnerApplicationService creates a new partner application service.
func NewPartnerApplicationService(
	repo domain.PartnerApplicationRepository,
	userRepo domain.UserRepository,
	partnerRepo domain.PartnerRepository,
	commercialRepo domain.CommercialRepository,
	pool *pgxpool.Pool,
) *PartnerApplicationService {
	return &PartnerApplicationService{
		repo:           repo,
		userRepo:       userRepo,
		partnerRepo:    partnerRepo,
		commercialRepo: commercialRepo,
		pool:           pool,
	}
}

// Submit saves a new partner application linked to the authenticated user.
func (s *PartnerApplicationService) Submit(ctx context.Context, ac *auth.AuthContext, storeName, fullName, phone, city, businessLocation, commercialReferralCode string) (*domain.PartnerApplication, *domain.AppError) {
	existing, err := s.repo.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if existing != nil && existing.Status == string(domain.PartnerAppStatusPending) {
		return nil, domain.Conflict("you already have a pending partner application")
	}
	if existing != nil && existing.Status == string(domain.PartnerAppStatusApproved) {
		return nil, domain.Conflict("you are already an approved partner")
	}

	var commercialID *uuid.UUID
	acquisitionSource := "direct"
	if commercialReferralCode != "" {
		if s.commercialRepo == nil {
			return nil, domain.BadRequest("commercial referral is not available")
		}
		profile, err := s.commercialRepo.GetProfileByReferralCode(ctx, ac.OrgID, normalizeReferralCode(commercialReferralCode))
		if err != nil {
			return nil, domain.InternalError(err)
		}
		if profile == nil || profile.Status != "active" {
			return nil, domain.NotFound("commercial referral")
		}
		commercialID = &profile.ID
		acquisitionSource = "commercial_referral_link"
	}

	app := &domain.PartnerApplication{
		OrgID:             ac.OrgID,
		UserID:            ac.UserID,
		StoreName:         storeName,
		FullName:          fullName,
		Phone:             phone,
		City:              city,
		BusinessLocation:  businessLocation,
		CommercialID:      commercialID,
		AcquisitionSource: acquisitionSource,
	}
	if commercialID == nil {
		if err := s.repo.Create(ctx, app); err != nil {
			return nil, domain.InternalError(err)
		}
		return app, nil
	}

	commissionPercentage := defaultCommercialPartnerCommissionPercentage
	if appErr := s.createAndApproveCommercialApplication(ctx, app, commissionPercentage); appErr != nil {
		return nil, appErr
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
	if app.Status != string(domain.PartnerAppStatusPending) &&
		!(app.Status == string(domain.PartnerAppStatusApproved) && decision == "rejected") {
		return nil, domain.Conflict("application has already been reviewed")
	}

	now := time.Now()
	app.ReviewedBy = &ac.UserID
	app.ReviewedAt = &now

	switch decision {
	case "rejected":
		app.Status = string(domain.PartnerAppStatusRejected)
		app.RejectionReason = rejectionReason

		if err := s.rejectApplication(ctx, app); err != nil {
			return nil, err
		}
		return app, nil

	case "approved":
		if !isValidCommissionPercentage(commissionPercentage) {
			return nil, domain.BadRequest("commission_percentage must be between 0 and 100 with up to 2 decimals")
		}
		app.Status = string(domain.PartnerAppStatusApproved)
		app.CommissionPercentage = commissionPercentage
		return app, s.createApprovedPartner(ctx, app, *commissionPercentage)

	default:
		return nil, domain.BadRequest("decision must be 'approved' or 'rejected'")
	}
}

func (s *PartnerApplicationService) rejectApplication(ctx context.Context, app *domain.PartnerApplication) *domain.AppError {
	if txErr := database.WithTransaction(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE partner_applications
			SET status = $2, reviewed_by = $3, rejection_reason = $4, reviewed_at = $5
			WHERE id = $1
		`, app.ID, app.Status, app.ReviewedBy, app.RejectionReason, app.ReviewedAt); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE partners
			SET status = 'inactive', updated_at = now()
			WHERE org_id = $1 AND user_id = $2
		`, app.OrgID, app.UserID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET role = CASE WHEN role = $2 THEN $3 ELSE role END,
				updated_at = now()
			WHERE id = $1 AND deleted_at IS NULL
		`, app.UserID, string(auth.RolePartner), string(auth.RoleMember)); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE "user"
			SET role = CASE WHEN role = $2 THEN $3 ELSE role END,
				"updatedAt" = now()
			WHERE id = (SELECT better_auth_id FROM users WHERE id = $1 AND deleted_at IS NULL)
		`, app.UserID, string(auth.RolePartner), string(auth.RoleMember)); err != nil {
			return err
		}

		return nil
	}); txErr != nil {
		return domain.InternalError(txErr)
	}
	return nil
}

// Returns *domain.AppError (not the error interface) so a nil result stays a
// true nil at the call site. Declaring `error` here would box a nil
// *domain.AppError into a non-nil interface, making Submit treat a SUCCESSFUL
// approval as a failure — and then panic in respond.Error calling .Error() on
// the nil pointer.
func (s *PartnerApplicationService) createAndApproveCommercialApplication(ctx context.Context, app *domain.PartnerApplication, commissionPercentage float64) *domain.AppError {
	now := time.Now()
	app.Status = string(domain.PartnerAppStatusApproved)
	app.ReviewedAt = &now
	app.CommissionPercentage = &commissionPercentage
	return s.createApprovedPartner(ctx, app, commissionPercentage)
}

func (s *PartnerApplicationService) createApprovedPartner(ctx context.Context, app *domain.PartnerApplication, commissionPercentage float64) *domain.AppError {
	referralCode, err := generateUniqueReferralCode(ctx, s.partnerRepo)
	if err != nil {
		return domain.InternalError(err)
	}

	if txErr := database.WithTransaction(ctx, s.pool, func(tx pgx.Tx) error {
		if app.ID == uuid.Nil {
			if err := tx.QueryRow(ctx, `
				INSERT INTO partner_applications (
					org_id, user_id, store_name, full_name, phone, city, business_location,
					commercial_id, acquisition_source, status, reviewed_by, rejection_reason, reviewed_at
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, $12)
				RETURNING id, status, created_at, reviewed_at
			`, app.OrgID, app.UserID, app.StoreName, app.FullName, app.Phone, app.City, app.BusinessLocation,
				app.CommercialID, app.AcquisitionSource, app.Status, app.ReviewedBy, app.ReviewedAt,
			).Scan(&app.ID, &app.Status, &app.CreatedAt, &app.ReviewedAt); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx, `
				UPDATE partner_applications
				SET status = $2, reviewed_by = $3, rejection_reason = $4, reviewed_at = $5
				WHERE id = $1
			`, app.ID, app.Status, app.ReviewedBy, app.RejectionReason, app.ReviewedAt); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO partners (
				org_id, user_id, store_name, city, business_location, referral_code, commission_percentage,
				commercial_id, acquisition_source, status
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'active')
			ON CONFLICT (org_id, user_id) DO UPDATE
			SET store_name = EXCLUDED.store_name,
				city = EXCLUDED.city,
				business_location = EXCLUDED.business_location,
				commission_percentage = EXCLUDED.commission_percentage,
				commercial_id = COALESCE(partners.commercial_id, EXCLUDED.commercial_id),
				acquisition_source = CASE
					WHEN partners.acquisition_source = 'direct' THEN EXCLUDED.acquisition_source
					ELSE partners.acquisition_source
				END,
				status = 'active',
				updated_at = now()
		`, app.OrgID, app.UserID, app.StoreName, app.City, app.BusinessLocation, referralCode, commissionPercentage, app.CommercialID, app.AcquisitionSource); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE users
			SET role = CASE
					WHEN role IN ('member', 'viewer') THEN $2
					ELSE role
				END,
				updated_at = now()
			WHERE id = $1 AND deleted_at IS NULL
		`, app.UserID, string(auth.RolePartner)); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE "user"
			SET role = CASE
					WHEN role IN ('member', 'viewer') THEN $2
					ELSE role
				END,
				"updatedAt" = now()
			WHERE id = (SELECT better_auth_id FROM users WHERE id = $1 AND deleted_at IS NULL)
		`, app.UserID, string(auth.RolePartner)); err != nil {
			return err
		}

		return nil
	}); txErr != nil {
		return domain.InternalError(txErr)
	}

	return nil
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
