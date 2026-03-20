package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// UserRepository defines data access for user profiles.
type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	Update(ctx context.Context, user *User) error
	UpdateRole(ctx context.Context, userID uuid.UUID, role string) error
}

// PlanRepository defines data access for insurance plans.
type PlanRepository interface {
	List(ctx context.Context) ([]Plan, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Plan, error)
	GetBySlug(ctx context.Context, slug string) (*Plan, error)
}

// DeviceRepository defines data access for registered devices.
type DeviceRepository interface {
	Create(ctx context.Context, device *Device) error
	GetByID(ctx context.Context, id uuid.UUID) (*Device, error)
	GetByIMEI(ctx context.Context, imei string) (*Device, error)
	ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]Device, error)
	Update(ctx context.Context, device *Device) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// SubscriptionRepository defines data access for subscriptions.
type SubscriptionRepository interface {
	Create(ctx context.Context, sub *Subscription) error
	GetByID(ctx context.Context, id uuid.UUID) (*Subscription, error)
	GetByDeviceID(ctx context.Context, deviceID uuid.UUID) (*Subscription, error)
	ListByDeviceID(ctx context.Context, deviceID uuid.UUID, limit int) ([]Subscription, error)
	ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]Subscription, error)
	Update(ctx context.Context, sub *Subscription) error
}

// ClaimRepository defines data access for insurance claims.
type ClaimRepository interface {
	Create(ctx context.Context, claim *Claim) error
	GetByID(ctx context.Context, id uuid.UUID) (*Claim, error)
	ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]Claim, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, status *string, limit, offset int) ([]Claim, error)
	Update(ctx context.Context, claim *Claim) error
	ExistsPendingByDeviceAndType(ctx context.Context, orgID, deviceID uuid.UUID, claimType ClaimType) (bool, error)
}

// ContactRepository defines data access for contact messages.
type ContactRepository interface {
	Create(ctx context.Context, msg *ContactMessage) error
}

// PartnerApplicationRepository defines data access for partner applications.
type PartnerApplicationRepository interface {
	Create(ctx context.Context, app *PartnerApplication) error
	GetByID(ctx context.Context, id uuid.UUID) (*PartnerApplication, error)
	GetByUser(ctx context.Context, orgID, userID uuid.UUID) (*PartnerApplication, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, status *string, limit, offset int) ([]AdminPartnerApplication, error)
	UpdateStatus(ctx context.Context, app *PartnerApplication) error
}

// AdminRepository provides aggregate queries for the admin dashboard.
type AdminRepository interface {
	GetStats(ctx context.Context, orgID uuid.UUID) (*AdminStats, error)
	ListCustomers(ctx context.Context, orgID uuid.UUID, search string, limit, offset int) ([]AdminCustomer, error)
	ListPayments(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]AdminPayment, error)
}

// PaymentRepository defines data access for payments.
type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*Payment, error)
	GetByProviderRef(ctx context.Context, providerRef string) (*Payment, error)
	GetFirstSuccessfulByUser(ctx context.Context, orgID, userID uuid.UUID) (*Payment, error)
	ListBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID, limit int) ([]Payment, error)
	ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]Payment, error)
	Update(ctx context.Context, payment *Payment) error
}

// PartnerRepository defines data access for the partner domain.
type PartnerRepository interface {
	Create(ctx context.Context, partner *Partner) error
	GetByID(ctx context.Context, partnerID uuid.UUID) (*Partner, error)
	GetByUser(ctx context.Context, orgID, userID uuid.UUID) (*Partner, error)
	GetProfile(ctx context.Context, orgID, userID uuid.UUID) (*PartnerProfile, error)
	CreateClient(ctx context.Context, client *PartnerClient) error
	GetClientByID(ctx context.Context, clientID uuid.UUID) (*PartnerClient, error)
	GetClientByLinkedUser(ctx context.Context, orgID, userID uuid.UUID) (*PartnerClient, error)
	GetClientByInvitationToken(ctx context.Context, token string) (*PartnerClient, error)
	GetInvitationDetailsByToken(ctx context.Context, token string) (*PartnerInvitationDetails, error)
	ListClients(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]PartnerClient, error)
	ClaimClientInvitation(ctx context.Context, clientID, userID uuid.UUID) error
	RefreshClientInvitation(ctx context.Context, clientID uuid.UUID, token string, expiresAt time.Time) error
	UpdateClientStatus(ctx context.Context, clientID uuid.UUID, status string, planID *uuid.UUID) error
	UpdateClientStatusByLinkedUser(ctx context.Context, userID uuid.UUID, status string, planID *uuid.UUID) error
	CreateCommission(ctx context.Context, commission *PartnerCommission) error
	ListSales(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]PartnerSale, error)
	ListPayouts(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]PartnerPayout, error)
	ListAll(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]AdminPartner, error)
	ListAdminCommissions(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]AdminPartnerCommission, error)
}

// WebhookEventRepository defines data access for webhook event dedup.
type WebhookEventRepository interface {
	Exists(ctx context.Context, idempotencyKey string) (bool, error)
	Create(ctx context.Context, event *WebhookEvent) error
}

// RepairRepository defines data access for repair bookings.
type RepairRepository interface {
	Create(ctx context.Context, booking *RepairBooking) error
	GetByID(ctx context.Context, id uuid.UUID) (*RepairBooking, error)
	GetByReferenceAndPhone(ctx context.Context, reference, normalizedPhone string) (*RepairBooking, error)
	ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]RepairBooking, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, status *string, search string, limit, offset int) ([]RepairBooking, error)
	Update(ctx context.Context, booking *RepairBooking) error
}
