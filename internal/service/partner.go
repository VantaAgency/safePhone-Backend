package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// PartnerService handles business logic for the partner domain.
type PartnerService struct {
	repo        domain.PartnerRepository
	userRepo    domain.UserRepository
	paymentRepo domain.PaymentRepository
	frontendURL string
}

// NewPartnerService creates a new partner service.
func NewPartnerService(repo domain.PartnerRepository, userRepo domain.UserRepository, paymentRepo domain.PaymentRepository, frontendURL string) *PartnerService {
	return &PartnerService{
		repo:        repo,
		userRepo:    userRepo,
		paymentRepo: paymentRepo,
		frontendURL: strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
	}
}

// GetProfile returns the partner profile with stats for the current user.
func (s *PartnerService) GetProfile(ctx context.Context, ac *auth.AuthContext) (*domain.PartnerProfile, *domain.AppError) {
	profile, err := s.repo.GetProfile(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if profile == nil {
		return nil, domain.NotFound("partner profile not found")
	}
	return profile, nil
}

// ListClients returns the client pipeline for the current partner.
func (s *PartnerService) ListClients(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.PartnerClient, *domain.AppError) {
	partner, err := s.repo.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil {
		return nil, domain.NotFound("partner profile not found")
	}

	clients, err := s.repo.ListClients(ctx, partner.ID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if clients == nil {
		clients = []domain.PartnerClient{}
	}
	for i := range clients {
		if invitationExpired(clients[i].InvitationExpiresAt) && clients[i].Status != "active" {
			clients[i].Status = "expired"
		}
		s.decorateClientInvitation(&clients[i])
	}
	return clients, nil
}

// CreateClient adds a new client to the partner's pipeline.
func (s *PartnerService) CreateClient(ctx context.Context, ac *auth.AuthContext, clientName, clientPhone string, planID *uuid.UUID) (*domain.PartnerClient, *domain.AppError) {
	partner, err := s.repo.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil {
		return nil, domain.NotFound("partner profile not found")
	}

	var phone *string
	if clientPhone != "" {
		phone = &clientPhone
	}

	client := &domain.PartnerClient{
		OrgID:               ac.OrgID,
		PartnerID:           partner.ID,
		ClientName:          clientName,
		ClientPhone:         phone,
		PlanID:              planID,
		Status:              "invited",
		AttributionSource:   "manual_invitation",
		ReferralCode:        ptrString(partner.ReferralCode),
		ReferralMedium:      "unknown",
		InvitationToken:     buildInvitationToken(),
		InvitationExpiresAt: ptrTime(invitationExpiresAt()),
	}
	if err := s.repo.CreateClient(ctx, client); err != nil {
		return nil, domain.InternalError(err)
	}
	s.decorateClientInvitation(client)
	return client, nil
}

// RefreshInvitation rotates a partner invitation link while keeping the client record.
func (s *PartnerService) RefreshInvitation(ctx context.Context, ac *auth.AuthContext, clientID uuid.UUID) (*domain.PartnerClient, *domain.AppError) {
	partner, err := s.repo.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil {
		return nil, domain.NotFound("partner profile not found")
	}

	client, err := s.repo.GetClientByID(ctx, clientID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if client == nil || client.PartnerID != partner.ID || client.OrgID != ac.OrgID {
		return nil, domain.NotFound("partner client not found")
	}

	token := buildInvitationToken()
	expiresAt := invitationExpiresAt()
	if err := s.repo.RefreshClientInvitation(ctx, clientID, token, expiresAt); err != nil {
		return nil, domain.InternalError(err)
	}

	client.InvitationToken = token
	client.InvitationExpiresAt = &expiresAt
	if client.Status == "draft" || client.Status == "expired" {
		client.Status = "invited"
	}
	s.decorateClientInvitation(client)
	return client, nil
}

// GetInvitationDetails resolves a public invitation token.
func (s *PartnerService) GetInvitationDetails(ctx context.Context, token string) (*domain.PartnerInvitationDetails, *domain.AppError) {
	details, err := s.repo.GetInvitationDetailsByToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if details == nil {
		return nil, domain.NotFound("partner invitation")
	}

	if invitationExpired(details.InvitationExpiresAt) && details.Status != "active" {
		details.Status = "expired"
	}
	details.InvitationURL = buildInvitationURL(s.frontendURL, token)
	return details, nil
}

// GetReferralDetails resolves a reusable public partner referral code.
func (s *PartnerService) GetReferralDetails(ctx context.Context, code string) (*domain.PartnerReferralDetails, *domain.AppError) {
	details, err := s.repo.GetReferralDetailsByCode(ctx, normalizeReferralCode(code))
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if details == nil || strings.TrimSpace(details.Status) != "active" {
		return nil, domain.NotFound("partner referral")
	}
	details.ReferralLink = buildReferralURL(s.frontendURL, details.ReferralCode)
	return details, nil
}

// TrackReferralVisit logs a reusable-link visit and returns the visitor token/context.
func (s *PartnerService) TrackReferralVisit(ctx context.Context, code, visitorToken, sourceMedium string) (*domain.PartnerReferralVisitResult, *domain.AppError) {
	partner, err := s.repo.GetByReferralCode(ctx, normalizeReferralCode(code))
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil || strings.TrimSpace(partner.Status) != "active" {
		return nil, domain.NotFound("partner referral")
	}

	token := strings.TrimSpace(visitorToken)
	if token == "" {
		token = buildVisitorToken()
	}

	visit := &domain.PartnerReferralVisit{
		OrgID:        partner.OrgID,
		PartnerID:    partner.ID,
		ReferralCode: partner.ReferralCode,
		VisitorToken: token,
		SourceMedium: normalizeReferralMedium(sourceMedium),
		VisitedAt:    time.Now(),
	}
	if err := s.repo.CreateReferralVisit(ctx, visit); err != nil {
		return nil, domain.InternalError(err)
	}

	return &domain.PartnerReferralVisitResult{
		Referral: &domain.PartnerReferralDetails{
			PartnerID:        partner.ID,
			PartnerStoreName: partner.StoreName,
			PartnerCity:      partner.City,
			ReferralCode:     partner.ReferralCode,
			ReferralLink:     buildReferralURL(s.frontendURL, partner.ReferralCode),
			Status:           partner.Status,
		},
		VisitorToken: token,
		SourceMedium: visit.SourceMedium,
		VisitedAt:    visit.VisitedAt,
	}, nil
}

// ClaimReferral links the authenticated account to a partner's reusable referral code.
func (s *PartnerService) ClaimReferral(ctx context.Context, ac *auth.AuthContext, code, sourceMedium string) (*domain.PartnerReferralDetails, *domain.AppError) {
	partner, err := s.repo.GetByReferralCode(ctx, normalizeReferralCode(code))
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil || partner.OrgID != ac.OrgID || strings.TrimSpace(partner.Status) != "active" {
		return nil, domain.NotFound("partner referral")
	}

	existingClient, err := s.repo.GetClientByLinkedUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if existingClient != nil {
		if existingClient.PartnerID != partner.ID {
			return nil, domain.Conflict("this account is already attributed to another partner")
		}
		return s.GetReferralDetails(ctx, partner.ReferralCode)
	}

	if s.paymentRepo != nil {
		firstSuccessfulPayment, err := s.paymentRepo.GetFirstSuccessfulByUser(ctx, ac.OrgID, ac.UserID)
		if err != nil {
			return nil, domain.InternalError(err)
		}
		if firstSuccessfulPayment != nil {
			return nil, domain.Conflict("partner attribution must be set before the first successful payment")
		}
	}

	if s.userRepo == nil {
		return nil, domain.InternalError(fmt.Errorf("missing user repository"))
	}

	user, err := s.userRepo.GetByID(ctx, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if user == nil || user.OrgID != ac.OrgID {
		return nil, domain.NotFound("user")
	}

	now := time.Now()
	client := &domain.PartnerClient{
		OrgID:               ac.OrgID,
		PartnerID:           partner.ID,
		LinkedUserID:        &ac.UserID,
		ClientName:          fallbackPartnerClientName(user),
		ClientPhone:         user.Phone,
		Status:              "account_created",
		AttributionSource:   "partner_referral_link",
		ReferralCode:        ptrString(partner.ReferralCode),
		ReferralMedium:      normalizeReferralMedium(sourceMedium),
		AttributedAt:        &now,
		InvitationToken:     buildInvitationToken(),
		InvitationExpiresAt: ptrTime(invitationExpiresAt()),
		InvitationClaimedAt: &now,
	}

	if err := s.repo.CreateClient(ctx, client); err != nil {
		raceWinner, lookupErr := s.repo.GetClientByLinkedUser(ctx, ac.OrgID, ac.UserID)
		if lookupErr == nil && raceWinner != nil {
			if raceWinner.PartnerID != partner.ID {
				return nil, domain.Conflict("this account is already attributed to another partner")
			}
			return s.GetReferralDetails(ctx, partner.ReferralCode)
		}
		return nil, domain.InternalError(err)
	}

	return s.GetReferralDetails(ctx, partner.ReferralCode)
}

// ClaimInvitation links the current user to an invitation after auth.
func (s *PartnerService) ClaimInvitation(ctx context.Context, ac *auth.AuthContext, token string) (*domain.PartnerInvitationDetails, *domain.AppError) {
	client, err := s.repo.GetClientByInvitationToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if client == nil {
		return nil, domain.NotFound("partner invitation")
	}
	if invitationExpired(client.InvitationExpiresAt) && client.Status != "active" {
		return nil, domain.Conflict("partner invitation has expired")
	}
	if client.LinkedUserID != nil && *client.LinkedUserID != ac.UserID {
		return nil, domain.Conflict("partner invitation is already linked to another account")
	}

	if err := s.repo.ClaimClientInvitation(ctx, client.ID, ac.UserID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.Conflict("partner invitation is already linked to another account")
		}
		return nil, domain.InternalError(err)
	}

	details, appErr := s.GetInvitationDetails(ctx, token)
	if appErr != nil {
		return nil, appErr
	}
	return details, nil
}

// UpdateClientStatus updates the status of a partner client (public, no auth required).
func (s *PartnerService) UpdateClientStatus(ctx context.Context, clientID uuid.UUID, status string, planID *uuid.UUID) *domain.AppError {
	if err := s.repo.UpdateClientStatus(ctx, clientID, status, planID); err != nil {
		return domain.InternalError(err)
	}
	return nil
}

// SyncClientStatusByUser advances the latest invitation linked to this user.
func (s *PartnerService) SyncClientStatusByUser(ctx context.Context, userID uuid.UUID, status string, planID *uuid.UUID) *domain.AppError {
	if err := s.repo.UpdateClientStatusByLinkedUser(ctx, userID, status, planID); err != nil {
		return domain.InternalError(err)
	}
	return nil
}

// ListSales returns recent sales/commissions for the current partner.
func (s *PartnerService) ListSales(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.PartnerSale, *domain.AppError) {
	partner, err := s.repo.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil {
		return nil, domain.NotFound("partner profile not found")
	}

	sales, err := s.repo.ListSales(ctx, partner.ID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if sales == nil {
		sales = []domain.PartnerSale{}
	}
	return sales, nil
}

// ListPayouts returns payout history for the current partner.
func (s *PartnerService) ListPayouts(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.PartnerPayout, *domain.AppError) {
	partner, err := s.repo.GetByUser(ctx, ac.OrgID, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil {
		return nil, domain.NotFound("partner profile not found")
	}

	payouts, err := s.repo.ListPayouts(ctx, partner.ID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if payouts == nil {
		payouts = []domain.PartnerPayout{}
	}
	return payouts, nil
}

// ListAll returns all partners for the admin dashboard.
func (s *PartnerService) ListAll(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.AdminPartner, *domain.AppError) {
	partners, err := s.repo.ListAll(ctx, ac.OrgID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partners == nil {
		partners = []domain.AdminPartner{}
	}
	return partners, nil
}

// ListAdminCommissions returns commission line items for a specific partner in the admin dashboard.
func (s *PartnerService) ListAdminCommissions(ctx context.Context, ac *auth.AuthContext, partnerID uuid.UUID, limit, offset int) ([]domain.AdminPartnerCommission, *domain.AppError) {
	partner, err := s.repo.GetByID(ctx, partnerID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil || partner.OrgID != ac.OrgID {
		return nil, domain.NotFound("partner")
	}

	commissions, err := s.repo.ListAdminCommissions(ctx, partnerID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if commissions == nil {
		commissions = []domain.AdminPartnerCommission{}
	}
	return commissions, nil
}

// ListAdminReferrals returns customer-level referral reporting for a partner in the admin dashboard.
func (s *PartnerService) ListAdminReferrals(ctx context.Context, ac *auth.AuthContext, partnerID uuid.UUID, limit, offset int) ([]domain.AdminPartnerReferral, *domain.AppError) {
	partner, err := s.repo.GetByID(ctx, partnerID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if partner == nil || partner.OrgID != ac.OrgID {
		return nil, domain.NotFound("partner")
	}

	items, err := s.repo.ListAdminReferrals(ctx, partnerID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if items == nil {
		items = []domain.AdminPartnerReferral{}
	}
	return items, nil
}

func (s *PartnerService) decorateClientInvitation(client *domain.PartnerClient) {
	if client == nil || strings.TrimSpace(client.InvitationToken) == "" {
		return
	}
	client.InvitationURL = buildInvitationURL(s.frontendURL, client.InvitationToken)
}

func buildInvitationToken() string {
	return uuid.NewString()
}

func buildVisitorToken() string {
	return uuid.NewString()
}

func buildReferralCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const length = 8

	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	var code strings.Builder
	code.Grow(length)
	for _, b := range buf {
		code.WriteByte(alphabet[int(b)%len(alphabet)])
	}
	return code.String(), nil
}

func generateUniqueReferralCode(ctx context.Context, repo domain.PartnerRepository) (string, error) {
	for range 12 {
		code, err := buildReferralCode()
		if err != nil {
			return "", err
		}
		existing, err := repo.GetByReferralCode(ctx, code)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return code, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique referral code")
}

func invitationExpiresAt() time.Time {
	return time.Now().Add(30 * 24 * time.Hour)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func ptrString(value string) *string {
	return &value
}

func invitationExpired(expiresAt *time.Time) bool {
	return expiresAt != nil && time.Now().After(*expiresAt)
}

func buildInvitationURL(frontendURL, token string) string {
	if frontendURL == "" || token == "" {
		return ""
	}
	return fmt.Sprintf("%s/inscription?invite=%s", frontendURL, token)
}

func buildReferralURL(frontendURL, code string) string {
	if frontendURL == "" || code == "" {
		return ""
	}
	return fmt.Sprintf("%s/p/%s", frontendURL, code)
}

func normalizeReferralCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func normalizeReferralMedium(sourceMedium string) string {
	switch strings.ToLower(strings.TrimSpace(sourceMedium)) {
	case "qr":
		return "qr"
	case "share":
		return "share"
	default:
		return "unknown"
	}
}

func fallbackPartnerClientName(user *domain.User) string {
	if user == nil {
		return "Client SafePhone"
	}
	if name := strings.TrimSpace(user.FullName); name != "" {
		return name
	}
	if email := strings.TrimSpace(user.Email); email != "" {
		return email
	}
	return "Client SafePhone"
}
