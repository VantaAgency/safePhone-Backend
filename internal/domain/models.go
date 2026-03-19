// Package domain defines core business types shared across all layers.
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Organization represents a tenant in the system.
type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents an authenticated user.
type User struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        uuid.UUID  `json:"org_id"`
	Email        string     `json:"email"`
	FullName     string     `json:"full_name"`
	Phone        *string    `json:"phone,omitempty"`
	Role         string     `json:"role"`
	BetterAuthID *string    `json:"better_auth_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// Plan represents an insurance plan tier.
type Plan struct {
	ID            uuid.UUID `json:"id"`
	Slug          string    `json:"slug"`
	NameFR        string    `json:"name_fr"`
	NameEN        string    `json:"name_en"`
	PriceMonthly  int       `json:"price_monthly"`
	PriceAnnual   int       `json:"price_annual"`
	Tier          string    `json:"tier"`
	DeviceRangeFR *string   `json:"device_range_fr,omitempty"`
	DeviceRangeEN *string   `json:"device_range_en,omitempty"`
	FeaturesFR    []string  `json:"features_fr"`
	FeaturesEN    []string  `json:"features_en"`
	NotCoveredFR  []string  `json:"not_covered_fr"`
	NotCoveredEN  []string  `json:"not_covered_en"`
	ServiceTime   string    `json:"service_time"`
	IsPopular     bool      `json:"is_popular"`
	SortOrder     int       `json:"sort_order"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const DevelopmentTestPlanSlug = "test-plan-dev"

// DeviceStatus enumerates possible device states.
type DeviceStatus string

const (
	DeviceStatusPending   DeviceStatus = "pending"
	DeviceStatusActive    DeviceStatus = "active"
	DeviceStatusExpired   DeviceStatus = "expired"
	DeviceStatusSuspended DeviceStatus = "suspended"
)

// Device represents a registered smartphone.
type Device struct {
	ID        uuid.UUID    `json:"id"`
	OrgID     uuid.UUID    `json:"org_id"`
	UserID    uuid.UUID    `json:"user_id"`
	Brand     string       `json:"brand"`
	Model     string       `json:"model"`
	IMEI      string       `json:"imei"`
	Status    DeviceStatus `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	DeletedAt *time.Time   `json:"deleted_at,omitempty"`
}

// SubscriptionStatus enumerates possible subscription states.
type SubscriptionStatus string

const (
	SubscriptionStatusPending   SubscriptionStatus = "pending"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusExpired   SubscriptionStatus = "expired"
)

// Subscription links a device to a plan with billing information.
type Subscription struct {
	ID                 uuid.UUID          `json:"id"`
	OrgID              uuid.UUID          `json:"org_id"`
	UserID             uuid.UUID          `json:"user_id"`
	DeviceID           uuid.UUID          `json:"device_id"`
	PlanID             uuid.UUID          `json:"plan_id"`
	Status             SubscriptionStatus `json:"status"`
	BillingCycle       string             `json:"billing_cycle"`
	CurrentPeriodStart *time.Time         `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time         `json:"current_period_end,omitempty"`
	CancelledAt        *time.Time         `json:"cancelled_at,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// ClaimType enumerates the types of insurance claims.
type ClaimType string

const (
	ClaimTypeScreen    ClaimType = "screen"
	ClaimTypeWater     ClaimType = "water"
	ClaimTypeTheft     ClaimType = "theft"
	ClaimTypeBreakdown ClaimType = "breakdown"
)

// ClaimStatus enumerates possible claim states.
type ClaimStatus string

const (
	ClaimStatusPending  ClaimStatus = "pending"
	ClaimStatusReview   ClaimStatus = "review"
	ClaimStatusApproved ClaimStatus = "approved"
	ClaimStatusRejected ClaimStatus = "rejected"
	ClaimStatusSettled  ClaimStatus = "settled"
)

// Claim represents an insurance claim filed by a user.
type Claim struct {
	ID             uuid.UUID   `json:"id"`
	OrgID          uuid.UUID   `json:"org_id"`
	UserID         uuid.UUID   `json:"user_id"`
	DeviceID       uuid.UUID   `json:"device_id"`
	SubscriptionID uuid.UUID   `json:"subscription_id"`
	ClaimType      ClaimType   `json:"claim_type"`
	Description    *string     `json:"description,omitempty"`
	Status         ClaimStatus `json:"status"`
	AmountXOF      *int        `json:"amount_xof,omitempty"`
	FiledAt        time.Time   `json:"filed_at"`
	ReviewedAt     *time.Time  `json:"reviewed_at,omitempty"`
	SettledAt      *time.Time  `json:"settled_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// ContactMessage represents a message submitted via the contact form.
type ContactMessage struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Subject   *string   `json:"subject,omitempty"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// PartnerApplicationStatus enumerates possible partner application states.
type PartnerApplicationStatus string

const (
	PartnerAppStatusPending  PartnerApplicationStatus = "pending"
	PartnerAppStatusApproved PartnerApplicationStatus = "approved"
	PartnerAppStatusRejected PartnerApplicationStatus = "rejected"
)

// PartnerApplication represents an application to become a SafePhone partner.
type PartnerApplication struct {
	ID              uuid.UUID  `json:"id"`
	OrgID           uuid.UUID  `json:"org_id"`
	UserID          uuid.UUID  `json:"user_id"`
	StoreName       string     `json:"store_name"`
	FullName        string     `json:"full_name"`
	Phone           string     `json:"phone"`
	City            string     `json:"city"`
	Status          string     `json:"status"`
	ReviewedBy      *uuid.UUID `json:"reviewed_by,omitempty"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
}

// AdminPartnerApplication is a read-only view for admin review, including applicant email.
type AdminPartnerApplication struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	UserID          string     `json:"user_id"`
	StoreName       string     `json:"store_name"`
	FullName        string     `json:"full_name"`
	Phone           string     `json:"phone"`
	Email           string     `json:"email"`
	City            string     `json:"city"`
	Status          string     `json:"status"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
}

// AdminStats aggregates platform-level statistics for the admin dashboard.
type AdminStats struct {
	ActiveSubscribers int            `json:"active_subscribers"`
	MonthlyRevenueXOF int            `json:"monthly_revenue_xof"`
	OpenClaims        int            `json:"open_claims"`
	RevenueByProvider map[string]int `json:"revenue_by_provider"`
	TotalCustomers    int            `json:"total_customers"`
	TotalDevices      int            `json:"total_devices"`
}

// AdminCustomer is a read-only view combining user + subscription + device data.
type AdminCustomer struct {
	ID                 string  `json:"id"`
	FullName           string  `json:"full_name"`
	Phone              *string `json:"phone,omitempty"`
	Email              string  `json:"email"`
	PlanNameFR         *string `json:"plan_name_fr,omitempty"`
	PlanNameEN         *string `json:"plan_name_en,omitempty"`
	DeviceCount        int     `json:"device_count"`
	SubscriptionStatus *string `json:"subscription_status,omitempty"`
}

// AdminPayment is a read-only view combining payment + user + plan data.
type AdminPayment struct {
	ID            string     `json:"id"`
	CustomerName  string     `json:"customer_name"`
	PlanNameFR    *string    `json:"plan_name_fr,omitempty"`
	PlanNameEN    *string    `json:"plan_name_en,omitempty"`
	AmountXOF     int        `json:"amount_xof"`
	Provider      string     `json:"provider"`
	PaymentMethod *string    `json:"payment_method,omitempty"`
	Status        string     `json:"status"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

const (
	RepairStatusPending    = "pending"
	RepairStatusAccepted   = "accepted"
	RepairStatusRejected   = "rejected"
	RepairStatusScheduled  = "scheduled"
	RepairStatusInProgress = "in_progress"
	RepairStatusCompleted  = "completed"
	RepairStatusCancelled  = "cancelled"

	RepairServiceModeCenter = "center"
	RepairServiceModeHome   = "home"

	RepairRequestSourcePublicVisitor = "public_visitor"
	RepairRequestSourceSafePhoneUser = "safephone_user"
)

// RepairBooking represents a MobiTech repair request.
type RepairBooking struct {
	ID                      uuid.UUID  `json:"id"`
	OrgID                   *uuid.UUID `json:"-"`
	UserID                  *uuid.UUID `json:"-"`
	Reference               string     `json:"reference"`
	DeviceBrand             string     `json:"device_brand"`
	DeviceModel             string     `json:"device_model"`
	RepairType              string     `json:"repair_type"`
	ServiceMode             string     `json:"service_mode"`
	CenterID                *string    `json:"center_id,omitempty"`
	PreferredDate           string     `json:"preferred_date"`
	PreferredTime           string     `json:"preferred_time"`
	ScheduledDate           *string    `json:"scheduled_date,omitempty"`
	ScheduledTime           *string    `json:"scheduled_time,omitempty"`
	CustomerName            string     `json:"customer_name"`
	CustomerPhone           string     `json:"customer_phone"`
	CustomerPhoneNormalized string     `json:"-"`
	Status                  string     `json:"status"`
	RepairAmountXOF         *int       `json:"repair_amount_xof,omitempty"`
	RequestSource           string     `json:"request_source"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

// PaymentMethod enumerates known provider-reported payment methods.
type PaymentMethod string

const (
	PaymentMethodWave        PaymentMethod = "wave"
	PaymentMethodOrangeMoney PaymentMethod = "orange_money"
	PaymentMethodFreeMoney   PaymentMethod = "free_money"
	PaymentMethodCard        PaymentMethod = "card"
)

// PaymentStatus enumerates possible payment states.
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusCancelled PaymentStatus = "cancelled"
	PaymentStatusExpired   PaymentStatus = "expired"
	PaymentStatusRefunded  PaymentStatus = "refunded"
)

// Payment represents a financial transaction for a subscription.
type Payment struct {
	ID              uuid.UUID       `json:"id"`
	OrgID           uuid.UUID       `json:"org_id"`
	UserID          uuid.UUID       `json:"user_id"`
	PlanID          uuid.UUID       `json:"plan_id"`
	SubscriptionID  uuid.UUID       `json:"subscription_id"`
	AmountXOF       int             `json:"amount_xof"`
	Currency        string          `json:"currency"`
	Provider        string          `json:"provider"`
	PaymentMethod   *string         `json:"payment_method,omitempty"`
	Status          PaymentStatus   `json:"status"`
	ProviderRef     *string         `json:"provider_ref,omitempty"`
	PaymentURL      *string         `json:"payment_url,omitempty"`
	IdempotencyKey  *string         `json:"idempotency_key,omitempty"`
	ProviderPayload json.RawMessage `json:"-"`
	PaidAt          *time.Time      `json:"paid_at,omitempty"`
	FailedAt        *time.Time      `json:"failed_at,omitempty"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// Partner represents a registered partner store.
type Partner struct {
	ID             uuid.UUID `json:"id"`
	OrgID          uuid.UUID `json:"org_id"`
	UserID         uuid.UUID `json:"user_id"`
	StoreName      string    `json:"store_name"`
	City           string    `json:"city"`
	CommissionRate float64   `json:"commission_rate"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PartnerClient represents a client invited by a partner.
type PartnerClient struct {
	ID                  uuid.UUID  `json:"id"`
	OrgID               uuid.UUID  `json:"org_id"`
	PartnerID           uuid.UUID  `json:"partner_id"`
	LinkedUserID        *uuid.UUID `json:"linked_user_id,omitempty"`
	ClientName          string     `json:"client_name"`
	ClientPhone         *string    `json:"client_phone,omitempty"`
	PlanID              *uuid.UUID `json:"plan_id,omitempty"`
	Status              string     `json:"status"`
	InvitationToken     string     `json:"-"`
	InvitationURL       string     `json:"invitation_url,omitempty"`
	InvitationExpiresAt *time.Time `json:"invitation_expires_at,omitempty"`
	InvitationClaimedAt *time.Time `json:"invitation_claimed_at,omitempty"`
	InvitedAt           time.Time  `json:"invited_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// PartnerInvitationDetails is the public onboarding context for an invited client.
type PartnerInvitationDetails struct {
	ClientID            uuid.UUID  `json:"client_id"`
	PartnerID           uuid.UUID  `json:"partner_id"`
	PartnerStoreName    string     `json:"partner_store_name"`
	PartnerCity         string     `json:"partner_city"`
	ClientName          string     `json:"client_name"`
	ClientPhone         *string    `json:"client_phone,omitempty"`
	PlanID              *uuid.UUID `json:"plan_id,omitempty"`
	PlanNameFR          *string    `json:"plan_name_fr,omitempty"`
	PlanNameEN          *string    `json:"plan_name_en,omitempty"`
	Status              string     `json:"status"`
	InvitationURL       string     `json:"invitation_url,omitempty"`
	InvitationExpiresAt *time.Time `json:"invitation_expires_at,omitempty"`
	InvitationClaimedAt *time.Time `json:"invitation_claimed_at,omitempty"`
	LinkedUserID        *uuid.UUID `json:"-"`
}

// PartnerCommission represents an earned commission record.
type PartnerCommission struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        uuid.UUID  `json:"org_id"`
	PartnerID    uuid.UUID  `json:"partner_id"`
	PaymentID    *uuid.UUID `json:"payment_id,omitempty"`
	AmountXOF    int        `json:"amount_xof"`
	Status       string     `json:"status"`
	PayoutMethod *string    `json:"payout_method,omitempty"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// PartnerProfile combines partner identity with aggregated stats.
type PartnerProfile struct {
	ID                 uuid.UUID `json:"id"`
	StoreName          string    `json:"store_name"`
	City               string    `json:"city"`
	CommissionRate     float64   `json:"commission_rate"`
	Status             string    `json:"status"`
	TotalClients       int       `json:"total_clients"`
	ActiveClients      int       `json:"active_clients"`
	PlansPurchased     int       `json:"plans_purchased"`
	MonthCommissionXOF int       `json:"month_commission_xof"`
}

// PartnerSale is a read-only view of a commission with payment details.
type PartnerSale struct {
	ID            string    `json:"id"`
	CustomerName  string    `json:"customer_name"`
	PlanNameFR    *string   `json:"plan_name_fr,omitempty"`
	PlanNameEN    *string   `json:"plan_name_en,omitempty"`
	AmountXOF     int       `json:"amount_xof"`
	CommissionXOF int       `json:"commission_xof"`
	Date          time.Time `json:"date"`
}

// PartnerPayout is a read-only view of a paid commission (payout record).
type PartnerPayout struct {
	ID           string    `json:"id"`
	AmountXOF    int       `json:"amount_xof"`
	PayoutMethod string    `json:"payout_method"`
	Status       string    `json:"status"`
	PaidAt       time.Time `json:"paid_at"`
}

// WebhookEvent records a processed webhook for idempotency.
type WebhookEvent struct {
	ID             uuid.UUID `json:"id"`
	Provider       string    `json:"provider"`
	EventType      string    `json:"event_type"`
	ProviderRef    string    `json:"provider_ref"`
	IdempotencyKey string    `json:"idempotency_key"`
	Payload        []byte    `json:"payload"`
	ProcessedAt    time.Time `json:"processed_at"`
}

// AdminPartner is a read-only view of a partner for the admin dashboard.
type AdminPartner struct {
	ID                  string `json:"id"`
	StoreName           string `json:"store_name"`
	OwnerName           string `json:"owner_name"`
	City                string `json:"city"`
	ClientsCount        int    `json:"clients_count"`
	ActiveClients       int    `json:"active_clients"`
	CommissionThisMonth int    `json:"commission_this_month"`
	Status              string `json:"status"`
	JoinedAt            string `json:"joined_at"`
}
