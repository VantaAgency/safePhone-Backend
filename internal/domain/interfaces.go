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
	GetEmployeeProfile(ctx context.Context, orgID, userID uuid.UUID) (*EmployeeProfile, error)
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
	ListByOrg(ctx context.Context, orgID uuid.UUID, status *string, market string, limit, offset int) ([]Claim, error)
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
	ListCustomers(ctx context.Context, orgID uuid.UUID, search, market string, limit, offset int) ([]AdminCustomer, error)
	ListPayments(ctx context.Context, orgID uuid.UUID, market string, limit, offset int) ([]AdminPayment, error)
	ListEmployees(ctx context.Context, orgID uuid.UUID, search string, status *EmployeeAccountStatus, sort string, limit, offset int) ([]AdminEmployeeListItem, error)
	GetEmployee(ctx context.Context, orgID, userID uuid.UUID) (*AdminEmployeeDetail, error)
	UpdateEmployeeStatus(ctx context.Context, orgID, userID, updatedBy uuid.UUID, status EmployeeAccountStatus, suspendedReason *string) (*EmployeeProfile, error)
}

// CommercialRepository provides commercial acquisition workflow persistence.
type CommercialRepository interface {
	CreateProfile(ctx context.Context, profile *CommercialProfile) error
	GetProfileByUser(ctx context.Context, orgID, userID uuid.UUID) (*CommercialProfile, error)
	GetProfileByID(ctx context.Context, orgID, commercialID uuid.UUID) (*CommercialProfile, error)
	GetProfileByReferralCode(ctx context.Context, orgID uuid.UUID, code string) (*CommercialProfile, error)
	ListPartners(ctx context.Context, orgID, commercialID uuid.UUID, limit, offset int) ([]CommercialPartner, error)
	ListCommissions(ctx context.Context, orgID, commercialID uuid.UUID, limit, offset int) ([]CommercialCommissionView, error)
	ListActivityReports(ctx context.Context, orgID uuid.UUID, commercialID *uuid.UUID, partnerID *uuid.UUID, limit, offset int) ([]CommercialActivityReportView, error)
	CreateActivityReport(ctx context.Context, report *CommercialActivityReport) error
	GetActivityReport(ctx context.Context, orgID, reportID uuid.UUID) (*CommercialActivityReport, error)
	ListAdminCommercials(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]AdminCommercialListItem, error)
	GetAdminCommercial(ctx context.Context, orgID, commercialID uuid.UUID) (*AdminCommercialDetail, error)
	UpdateStatus(ctx context.Context, orgID, commercialID uuid.UUID, status string) (*CommercialProfile, error)
	UpdateCommissionPercentage(ctx context.Context, orgID, commercialID uuid.UUID, percentage float64) (*CommercialProfile, error)
	CreateCommissionForFirstPartnerPayment(ctx context.Context, commission *CommercialCommission) error
}

// EmployeeRepository provides operational read models and workflow persistence for employees.
type EmployeeRepository interface {
	GetOverviewMetrics(ctx context.Context, orgID uuid.UUID) (*EmployeeOverviewMetrics, error)
	ListClients(ctx context.Context, orgID uuid.UUID, search string, limit, offset int) ([]EmployeeClientListItem, error)
	GetClient(ctx context.Context, orgID, userID uuid.UUID) (*EmployeeClientDetail, error)
	ListPaymentFollowUps(ctx context.Context, orgID uuid.UUID, search string, limit, offset int) ([]EmployeePaymentFollowUpItem, error)
	ListClaims(ctx context.Context, orgID uuid.UUID, status *string, search string, limit, offset int) ([]EmployeeClaimDetail, error)
	GetClaim(ctx context.Context, orgID, claimID uuid.UUID) (*EmployeeClaimDetail, error)
	ListRepairs(ctx context.Context, orgID uuid.UUID, status *string, search string, limit, offset int) ([]EmployeeRepairDetail, error)
	GetRepair(ctx context.Context, orgID, repairID uuid.UUID) (*EmployeeRepairDetail, error)
	ListTasks(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]EmployeeTaskItem, error)
	GetFollowUp(ctx context.Context, orgID uuid.UUID, entityType OperationalEntityType, entityID uuid.UUID) (*OperationalFollowUp, error)
	UpsertFollowUp(ctx context.Context, followUp *OperationalFollowUp) error
	ListNotes(ctx context.Context, orgID uuid.UUID, entityType OperationalEntityType, entityID uuid.UUID, limit, offset int) ([]OperationalNote, error)
	CreateNote(ctx context.Context, note *OperationalNote) error
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
	GetByReferralCode(ctx context.Context, code string) (*Partner, error)
	GetProfile(ctx context.Context, orgID, userID uuid.UUID) (*PartnerProfile, error)
	CreateClient(ctx context.Context, client *PartnerClient) error
	GetClientByID(ctx context.Context, clientID uuid.UUID) (*PartnerClient, error)
	GetClientByLinkedUser(ctx context.Context, orgID, userID uuid.UUID) (*PartnerClient, error)
	GetClientByInvitationToken(ctx context.Context, token string) (*PartnerClient, error)
	GetInvitationDetailsByToken(ctx context.Context, token string) (*PartnerInvitationDetails, error)
	GetReferralDetailsByCode(ctx context.Context, code string) (*PartnerReferralDetails, error)
	ListClients(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]PartnerClient, error)
	ClaimClientInvitation(ctx context.Context, clientID, userID uuid.UUID) error
	RefreshClientInvitation(ctx context.Context, clientID uuid.UUID, token string, expiresAt time.Time) error
	UpdateClientStatus(ctx context.Context, clientID uuid.UUID, status string, planID *uuid.UUID) error
	UpdateClientStatusByLinkedUser(ctx context.Context, userID uuid.UUID, status string, planID *uuid.UUID) error
	CreateReferralVisit(ctx context.Context, visit *PartnerReferralVisit) error
	GetReferralMetrics(ctx context.Context, partnerID uuid.UUID) (*PartnerReferralMetrics, error)
	ListPlanBreakdown(ctx context.Context, partnerID uuid.UUID, limit int) ([]PartnerPlanBreakdown, error)
	CreateCommission(ctx context.Context, commission *PartnerCommission) error
	ListSales(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]PartnerSale, error)
	ListPayouts(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]PartnerPayout, error)
	ListAll(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]AdminPartner, error)
	ListAdminCommissions(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]AdminPartnerCommission, error)
	ListAdminReferrals(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]AdminPartnerReferral, error)
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
	ListByOrg(ctx context.Context, orgID uuid.UUID, status *string, search, market string, limit, offset int) ([]RepairBooking, error)
	Update(ctx context.Context, booking *RepairBooking) error
}
