package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

const maxCommercialActivityPhotoBytes = 8 << 20

var allowedCommercialActivityTypes = map[string]bool{
	"partner_registration": true,
	"partner_onboarding":   true,
	"partner_training":     true,
	"follow_up_visit":      true,
	"sales_meeting":        true,
	"document_collection":  true,
	"support_visit":        true,
}

// CommercialService coordinates commercial acquisition workflows.
type CommercialService struct {
	repo        domain.CommercialRepository
	partnerRepo domain.PartnerRepository
	frontendURL string
	uploadRoot  string
}

func NewCommercialService(repo domain.CommercialRepository, partnerRepo domain.PartnerRepository, frontendURL string) *CommercialService {
	return &CommercialService{
		repo:        repo,
		partnerRepo: partnerRepo,
		frontendURL: strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
		uploadRoot:  filepath.Join("uploads", "commercial-activity"),
	}
}

func (s *CommercialService) GetReferralDetails(ctx context.Context, orgID uuid.UUID, code string) (*domain.CommercialProfile, *domain.AppError) {
	profile, err := s.repo.GetProfileByReferralCode(ctx, orgID, normalizeReferralCode(code))
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if profile == nil || profile.Status != "active" {
		return nil, domain.NotFound("commercial referral")
	}
	return profile, nil
}

func (s *CommercialService) GetOverview(ctx context.Context, ac *auth.AuthContext) (*domain.CommercialDashboardOverview, *domain.AppError) {
	profile, appErr := s.requireCommercialProfile(ctx, ac)
	if appErr != nil {
		return nil, appErr
	}

	partners, err := s.repo.ListPartners(ctx, ac.OrgID, profile.ID, 8, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	reports, err := s.repo.ListActivityReports(ctx, ac.OrgID, &profile.ID, nil, 8, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	commissions, err := s.repo.ListCommissions(ctx, ac.OrgID, profile.ID, 8, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	allCommissions, err := s.repo.ListCommissions(ctx, ac.OrgID, profile.ID, 500, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}

	allPartners, err := s.repo.ListPartners(ctx, ac.OrgID, profile.ID, 500, 0)
	if err != nil {
		return nil, domain.InternalError(err)
	}

	overview := &domain.CommercialDashboardOverview{
		Profile:           *profile,
		ReferralLink:      buildCommercialReferralURL(s.frontendURL, profile.ReferralCode),
		RecentPartners:    partners,
		RecentReports:     reports,
		RecentCommissions: commissions,
	}
	for _, partner := range allPartners {
		overview.PartnersBrought++
		if partner.ApplicationStatus != nil && *partner.ApplicationStatus == "pending" {
			overview.PendingPartnerApplications++
		}
		if partner.Status == "active" || partner.Status == "approved" {
			overview.ApprovedPartners++
		}
		if partner.Status == "active" {
			overview.ActivePartners++
		}
		if partner.FirstPaymentStatus != nil {
			overview.FirstClientConversions++
		}
	}
	for _, commission := range allCommissions {
		overview.CommissionEarnedXOF += commission.CommissionAmountXOF
		if commission.Status != "paid" {
			overview.CommissionPendingXOF += commission.CommissionAmountXOF
		}
	}

	return overview, nil
}

func (s *CommercialService) ListPartners(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.CommercialPartner, *domain.AppError) {
	profile, appErr := s.requireCommercialProfile(ctx, ac)
	if appErr != nil {
		return nil, appErr
	}
	partners, err := s.repo.ListPartners(ctx, ac.OrgID, profile.ID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return partners, nil
}

func (s *CommercialService) ListCommissions(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.CommercialCommissionView, *domain.AppError) {
	profile, appErr := s.requireCommercialProfile(ctx, ac)
	if appErr != nil {
		return nil, appErr
	}
	items, err := s.repo.ListCommissions(ctx, ac.OrgID, profile.ID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

func (s *CommercialService) ListActivityReports(ctx context.Context, ac *auth.AuthContext, partnerID *uuid.UUID, limit, offset int) ([]domain.CommercialActivityReportView, *domain.AppError) {
	profile, appErr := s.requireCommercialProfile(ctx, ac)
	if appErr != nil {
		return nil, appErr
	}
	items, err := s.repo.ListActivityReports(ctx, ac.OrgID, &profile.ID, partnerID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

type ActivityPhotoInput struct {
	Reader      io.Reader
	FileName    string
	ContentType string
}

type CreateActivityReportInput struct {
	PartnerID    *uuid.UUID
	ProspectName *string
	ActivityType string
	Comment      string
	City         *string
	Location     *string
	Photo        ActivityPhotoInput
}

func (s *CommercialService) CreateActivityReport(ctx context.Context, ac *auth.AuthContext, input CreateActivityReportInput) (*domain.CommercialActivityReport, *domain.AppError) {
	profile, appErr := s.requireCommercialProfile(ctx, ac)
	if appErr != nil {
		return nil, appErr
	}

	activityType := strings.TrimSpace(input.ActivityType)
	if !allowedCommercialActivityTypes[activityType] {
		return nil, domain.BadRequest("invalid activity type")
	}
	comment := strings.TrimSpace(input.Comment)
	if comment == "" {
		return nil, domain.BadRequest("comment is required")
	}
	if input.PartnerID != nil && s.partnerRepo != nil {
		partner, err := s.partnerRepo.GetByID(ctx, *input.PartnerID)
		if err != nil {
			return nil, domain.InternalError(err)
		}
		if partner == nil || partner.OrgID != ac.OrgID || partner.CommercialID == nil || *partner.CommercialID != profile.ID {
			return nil, domain.NotFound("partner")
		}
	}

	storagePath, contentType, err := s.storeActivityPhoto(input.Photo)
	if err != nil {
		return nil, domain.BadRequest(err.Error())
	}

	reportID := uuid.New()
	report := &domain.CommercialActivityReport{
		ID:               reportID,
		OrgID:            ac.OrgID,
		CommercialID:     profile.ID,
		PartnerID:        input.PartnerID,
		ProspectName:     trimOptional(input.ProspectName),
		ActivityType:     activityType,
		Comment:          comment,
		City:             trimOptional(input.City),
		Location:         trimOptional(input.Location),
		PhotoStoragePath: storagePath,
		PhotoContentType: contentType,
		PhotoURL:         fmt.Sprintf("/api/v1/commercial/activity-reports/%s/photo", reportID.String()),
	}
	if err := s.repo.CreateActivityReport(ctx, report); err != nil {
		return nil, domain.InternalError(err)
	}
	report.ID = reportID
	return report, nil
}

func (s *CommercialService) GetActivityReportPhoto(ctx context.Context, ac *auth.AuthContext, reportID uuid.UUID) (*domain.CommercialActivityReport, *domain.AppError) {
	report, err := s.repo.GetActivityReport(ctx, ac.OrgID, reportID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if report == nil {
		return nil, domain.NotFound("activity report")
	}
	if ac.HasRole(auth.RoleAdmin) {
		return report, nil
	}
	profile, appErr := s.requireCommercialProfile(ctx, ac)
	if appErr != nil {
		return nil, appErr
	}
	if report.CommercialID != profile.ID {
		return nil, domain.Forbidden("activity report belongs to another commercial")
	}
	return report, nil
}

func (s *CommercialService) AdminListCommercials(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.AdminCommercialListItem, *domain.AppError) {
	items, err := s.repo.ListAdminCommercials(ctx, ac.OrgID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

func (s *CommercialService) AdminGetCommercial(ctx context.Context, ac *auth.AuthContext, commercialID uuid.UUID) (*domain.AdminCommercialDetail, *domain.AppError) {
	item, err := s.repo.GetAdminCommercial(ctx, ac.OrgID, commercialID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if item == nil {
		return nil, domain.NotFound("commercial")
	}
	return item, nil
}

func (s *CommercialService) AdminListActivityReports(ctx context.Context, ac *auth.AuthContext, commercialID *uuid.UUID, partnerID *uuid.UUID, limit, offset int) ([]domain.CommercialActivityReportView, *domain.AppError) {
	items, err := s.repo.ListActivityReports(ctx, ac.OrgID, commercialID, partnerID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	return items, nil
}

func (s *CommercialService) AdminUpdateStatus(ctx context.Context, ac *auth.AuthContext, commercialID uuid.UUID, status string) (*domain.CommercialProfile, *domain.AppError) {
	if status != "active" && status != "inactive" {
		return nil, domain.BadRequest("status must be active or inactive")
	}
	profile, err := s.repo.UpdateStatus(ctx, ac.OrgID, commercialID, status)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if profile == nil {
		return nil, domain.NotFound("commercial")
	}
	return profile, nil
}

func (s *CommercialService) AdminUpdateCommissionPercentage(ctx context.Context, ac *auth.AuthContext, commercialID uuid.UUID, percentage float64) (*domain.CommercialProfile, *domain.AppError) {
	if !validPercentage(percentage) {
		return nil, domain.BadRequest("commission_percentage must be between 0 and 100 with up to 2 decimals")
	}
	profile, err := s.repo.UpdateCommissionPercentage(ctx, ac.OrgID, commercialID, percentage)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if profile == nil {
		return nil, domain.NotFound("commercial")
	}
	return profile, nil
}

func (s *CommercialService) requireCommercialProfile(ctx context.Context, ac *auth.AuthContext) (*domain.CommercialProfile, *domain.AppError) {
	profile, err := s.repo.GetProfileByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if profile == nil {
		return nil, domain.NotFound("commercial profile")
	}
	if profile.Status != "active" {
		return nil, domain.Forbidden("commercial workspace access is disabled")
	}
	return profile, nil
}

func (s *CommercialService) storeActivityPhoto(photo ActivityPhotoInput) (string, string, error) {
	if photo.Reader == nil {
		return "", "", fmt.Errorf("photo is required")
	}

	limited := io.LimitReader(photo.Reader, maxCommercialActivityPhotoBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", "", fmt.Errorf("failed to read photo")
	}
	if len(data) == 0 {
		return "", "", fmt.Errorf("photo is required")
	}
	if len(data) > maxCommercialActivityPhotoBytes {
		return "", "", fmt.Errorf("photo must be 8MB or smaller")
	}

	contentType := strings.TrimSpace(photo.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	ext := ".jpg"
	switch contentType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	default:
		return "", "", fmt.Errorf("photo must be JPEG, PNG, or WebP")
	}

	if err := os.MkdirAll(s.uploadRoot, 0o750); err != nil {
		return "", "", fmt.Errorf("failed to prepare upload storage")
	}
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + ext
	path := filepath.Join(s.uploadRoot, name)
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return "", "", fmt.Errorf("failed to store photo")
	}
	return path, contentType, nil
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func buildCommercialReferralURL(frontendURL, code string) string {
	base := strings.TrimRight(frontendURL, "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	return fmt.Sprintf("%s/partenaires?commercial=%s", base, code)
}

func validPercentage(value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	if value <= 0 || value > 100 {
		return false
	}
	return math.Abs((value*100)-math.Round(value*100)) < 1e-9
}

func generateCommercialReferralCode() (string, error) {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes[:])), nil
}
