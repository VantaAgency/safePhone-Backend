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
	Market       MarketCode `json:"market"`
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
const TotalPlanSlug = "totale"

// DeviceType enumerates supported registered device categories.
type DeviceType string

const (
	DeviceTypeSmartphone      DeviceType = "smartphone"
	DeviceTypeTablet          DeviceType = "tablet"
	DeviceTypeTV              DeviceType = "tv"
	DeviceTypeComputer        DeviceType = "computer"
	DeviceTypeHomeElectronics DeviceType = "home_electronics"
)

// DeviceStatus enumerates possible device states.
type DeviceStatus string

const (
	DeviceStatusPending   DeviceStatus = "pending"
	DeviceStatusActive    DeviceStatus = "active"
	DeviceStatusExpired   DeviceStatus = "expired"
	DeviceStatusSuspended DeviceStatus = "suspended"
)

// DeviceMetadata stores type-specific optional fields for a registered device.
type DeviceMetadata struct {
	SerialNumber     string `json:"serial_number,omitempty"`
	ScreenSize       string `json:"screen_size,omitempty"`
	ComputerCategory string `json:"computer_category,omitempty"`
	ProductSubtype   string `json:"product_subtype,omitempty"`
}

// Device represents a registered covered device.
type Device struct {
	ID         uuid.UUID      `json:"id"`
	OrgID      uuid.UUID      `json:"org_id"`
	UserID     uuid.UUID      `json:"user_id"`
	DeviceType DeviceType     `json:"device_type"`
	Brand      string         `json:"brand"`
	Model      string         `json:"model"`
	Metadata   DeviceMetadata `json:"metadata"`
	IMEI       string         `json:"imei"`
	Status     DeviceStatus   `json:"status"`
	Market     MarketCode     `json:"market"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  *time.Time     `json:"deleted_at,omitempty"`
}

// SubscriptionStatus enumerates possible subscription states.
type SubscriptionStatus string

const (
	SubscriptionStatusPending   SubscriptionStatus = "pending"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusExpired   SubscriptionStatus = "expired"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
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
	Market             MarketCode         `json:"market"`
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
	AmountMinor    *int        `json:"amount_minor,omitempty"`
	Market         MarketCode  `json:"market"`
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
	ID                   uuid.UUID  `json:"id"`
	OrgID                uuid.UUID  `json:"org_id"`
	UserID               uuid.UUID  `json:"user_id"`
	StoreName            string     `json:"store_name"`
	FullName             string     `json:"full_name"`
	Phone                string     `json:"phone"`
	City                 string     `json:"city"`
	BusinessLocation     string     `json:"business_location"`
	Status               string     `json:"status"`
	CommissionPercentage *float64   `json:"commission_percentage,omitempty"`
	CommercialID         *uuid.UUID `json:"commercial_id,omitempty"`
	CommercialName       *string    `json:"commercial_name,omitempty"`
	AcquisitionSource    string     `json:"acquisition_source"`
	ReviewedBy           *uuid.UUID `json:"reviewed_by,omitempty"`
	RejectionReason      *string    `json:"rejection_reason,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	ReviewedAt           *time.Time `json:"reviewed_at,omitempty"`
}

// AdminPartnerApplication is a read-only view for admin review, including applicant email.
type AdminPartnerApplication struct {
	ID                   string     `json:"id"`
	OrgID                string     `json:"org_id"`
	UserID               string     `json:"user_id"`
	StoreName            string     `json:"store_name"`
	FullName             string     `json:"full_name"`
	Phone                string     `json:"phone"`
	Email                string     `json:"email"`
	City                 string     `json:"city"`
	BusinessLocation     string     `json:"business_location"`
	Status               string     `json:"status"`
	CommissionPercentage *float64   `json:"commission_percentage,omitempty"`
	CommercialID         *string    `json:"commercial_id,omitempty"`
	CommercialName       *string    `json:"commercial_name,omitempty"`
	AcquisitionSource    string     `json:"acquisition_source"`
	RejectionReason      *string    `json:"rejection_reason,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	ReviewedAt           *time.Time `json:"reviewed_at,omitempty"`
}

// ProviderRevenue is one row of the per-(provider × market) revenue breakdown.
// AmountMinor is stored in the minor unit appropriate for the market's
// currency (cents for USD, whole units for XOF) — derive currency at display
// time via CurrencyForMarket(market).
type ProviderRevenue struct {
	Provider    string     `json:"provider"`
	Market      MarketCode `json:"market"`
	AmountMinor int        `json:"amount_minor"`
}

// AdminStats aggregates platform-level statistics for the admin dashboard.
type AdminStats struct {
	ActiveSubscribers int               `json:"active_subscribers"`
	MonthlyRevenueXOF int               `json:"monthly_revenue_xof"`
	OpenClaims        int               `json:"open_claims"`
	RevenueByProvider []ProviderRevenue `json:"revenue_by_provider"`
	TotalCustomers    int               `json:"total_customers"`
	TotalDevices      int               `json:"total_devices"`
}

type DashboardCoverageStatus string

const (
	DashboardCoverageStatusActive            DashboardCoverageStatus = "active"
	DashboardCoverageStatusAwaitingPayment   DashboardCoverageStatus = "awaiting_payment"
	DashboardCoverageStatusPendingActivation DashboardCoverageStatus = "pending_activation"
	DashboardCoverageStatusPending           DashboardCoverageStatus = "pending"
	DashboardCoverageStatusFailed            DashboardCoverageStatus = "failed"
	DashboardCoverageStatusCancelled         DashboardCoverageStatus = "cancelled"
	DashboardCoverageStatusExpired           DashboardCoverageStatus = "expired"
	DashboardCoverageStatusRefunded          DashboardCoverageStatus = "refunded"
	DashboardCoverageStatusSuspended         DashboardCoverageStatus = "suspended"
)

type MemberDashboardDevice struct {
	Device         Device                  `json:"device"`
	CoverageStatus DashboardCoverageStatus `json:"coverage_status"`
	Subscription   *Subscription           `json:"subscription,omitempty"`
	Payment        *Payment                `json:"payment,omitempty"`
}

type MemberDashboardActiveSubscription struct {
	Subscription Subscription `json:"subscription"`
	Device       *Device      `json:"device,omitempty"`
}

type MemberDashboardSummary struct {
	ActiveSubscriptionsCount int                                 `json:"active_subscriptions_count"`
	DevicesCount             int                                 `json:"devices_count"`
	ClaimsCount              int                                 `json:"claims_count"`
	PaymentsCount            int                                 `json:"payments_count"`
	PendingActivationDevices []Device                            `json:"pending_activation_devices"`
	RecentDevices            []MemberDashboardDevice             `json:"recent_devices"`
	RecentClaims             []Claim                             `json:"recent_claims"`
	RecentPayments           []Payment                           `json:"recent_payments"`
	ActiveSubscriptions      []MemberDashboardActiveSubscription `json:"active_subscriptions"`
}

type AdminDashboardOverview struct {
	Stats         AdminStats      `json:"stats"`
	RecentClaims  []Claim         `json:"recent_claims"`
	RecentRepairs []RepairBooking `json:"recent_repairs"`
}

type EmployeeAccountStatus string

const (
	EmployeeAccountStatusActive    EmployeeAccountStatus = "active"
	EmployeeAccountStatusInactive  EmployeeAccountStatus = "inactive"
	EmployeeAccountStatusSuspended EmployeeAccountStatus = "suspended"
)

type EmployeeProfile struct {
	UserID          uuid.UUID             `json:"user_id"`
	OrgID           uuid.UUID             `json:"org_id"`
	Status          EmployeeAccountStatus `json:"status"`
	SuspendedReason *string               `json:"suspended_reason,omitempty"`
	CreatedBy       *uuid.UUID            `json:"created_by,omitempty"`
	UpdatedBy       *uuid.UUID            `json:"updated_by,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

type OperationalEntityType string

const (
	OperationalEntityTypeClient       OperationalEntityType = "client"
	OperationalEntityTypeSubscription OperationalEntityType = "subscription"
	OperationalEntityTypeClaim        OperationalEntityType = "claim"
	OperationalEntityTypeRepair       OperationalEntityType = "repair"
)

type FollowUpStatus string

const (
	FollowUpStatusToContact        FollowUpStatus = "to_contact"
	FollowUpStatusContacted        FollowUpStatus = "contacted"
	FollowUpStatusAwaitingResponse FollowUpStatus = "awaiting_response"
	FollowUpStatusResolved         FollowUpStatus = "resolved"
)

type PaymentFollowUpContext string

const (
	PaymentFollowUpContextFirstPayment PaymentFollowUpContext = "first_payment"
	PaymentFollowUpContextRenewal      PaymentFollowUpContext = "renewal"
)

type OperationalFollowUp struct {
	ID            uuid.UUID             `json:"id"`
	OrgID         uuid.UUID             `json:"org_id"`
	EntityType    OperationalEntityType `json:"entity_type"`
	EntityID      uuid.UUID             `json:"entity_id"`
	Reason        *string               `json:"reason,omitempty"`
	Status        FollowUpStatus        `json:"status"`
	NextAction    *string               `json:"next_action,omitempty"`
	LastContactAt *time.Time            `json:"last_contact_at,omitempty"`
	CreatedBy     *uuid.UUID            `json:"created_by,omitempty"`
	UpdatedBy     *uuid.UUID            `json:"updated_by,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

type OperationalNote struct {
	ID            uuid.UUID             `json:"id"`
	OrgID         uuid.UUID             `json:"org_id"`
	EntityType    OperationalEntityType `json:"entity_type"`
	EntityID      uuid.UUID             `json:"entity_id"`
	Body          string                `json:"body"`
	CreatedBy     uuid.UUID             `json:"created_by"`
	CreatedByName *string               `json:"created_by_name,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
}

type EmployeeOverviewMetrics struct {
	UnpaidSubscriptionsCount    int `json:"unpaid_subscriptions_count"`
	PendingPaymentsCount        int `json:"pending_payments_count"`
	FailedPaymentsCount         int `json:"failed_payments_count"`
	ClientsNeedingFollowUpCount int `json:"clients_needing_follow_up_count"`
	PendingClaimsCount          int `json:"pending_claims_count"`
	RepairsInProgressCount      int `json:"repairs_in_progress_count"`
	OverdueRepairsCount         int `json:"overdue_repairs_count"`
	PendingActivationCount      int `json:"pending_activation_count"`
	MissingIMEICount            int `json:"missing_imei_count"`
	UrgentTasksCount            int `json:"urgent_tasks_count"`
}

type EmployeeClientListItem struct {
	ID                      uuid.UUID               `json:"id"`
	FullName                string                  `json:"full_name"`
	Email                   string                  `json:"email"`
	Phone                   *string                 `json:"phone,omitempty"`
	DeviceCount             int                     `json:"device_count"`
	ActiveSubscriptionCount int                     `json:"active_subscription_count"`
	MissingIMEICount        int                     `json:"missing_imei_count"`
	PendingClaimsCount      int                     `json:"pending_claims_count"`
	OpenRepairsCount        int                     `json:"open_repairs_count"`
	LatestCoverageStatus    DashboardCoverageStatus `json:"latest_coverage_status"`
	PartnerStoreName        *string                 `json:"partner_store_name,omitempty"`
	RequiresAttention       bool                    `json:"requires_attention"`
	FollowUp                *OperationalFollowUp    `json:"follow_up,omitempty"`
}

type EmployeeClientDeviceCoverage struct {
	Device           Device                  `json:"device"`
	CoverageStatus   DashboardCoverageStatus `json:"coverage_status"`
	Subscription     *Subscription           `json:"subscription,omitempty"`
	Payment          *Payment                `json:"payment,omitempty"`
	PlanNameFR       *string                 `json:"plan_name_fr,omitempty"`
	PlanNameEN       *string                 `json:"plan_name_en,omitempty"`
	PartnerStoreName *string                 `json:"partner_store_name,omitempty"`
}

type EmployeePaymentFollowUpItem struct {
	UserID            uuid.UUID               `json:"user_id"`
	ClientName        string                  `json:"client_name"`
	ClientEmail       string                  `json:"client_email"`
	ClientPhone       *string                 `json:"client_phone,omitempty"`
	Device            Device                  `json:"device"`
	Subscription      *Subscription           `json:"subscription,omitempty"`
	Payment           *Payment                `json:"payment,omitempty"`
	CoverageStatus    DashboardCoverageStatus `json:"coverage_status"`
	PlanNameFR        *string                 `json:"plan_name_fr,omitempty"`
	PlanNameEN        *string                 `json:"plan_name_en,omitempty"`
	PaymentContext    PaymentFollowUpContext  `json:"payment_context"`
	RequiresAttention bool                    `json:"requires_attention"`
	AttentionReason   string                  `json:"attention_reason"`
	PartnerStoreName  *string                 `json:"partner_store_name,omitempty"`
	FollowUp          *OperationalFollowUp    `json:"follow_up,omitempty"`
}

type EmployeeClaimDetail struct {
	Claim              Claim                   `json:"claim"`
	ClientName         string                  `json:"client_name"`
	ClientEmail        string                  `json:"client_email"`
	ClientPhone        *string                 `json:"client_phone,omitempty"`
	DeviceBrand        string                  `json:"device_brand"`
	DeviceModel        string                  `json:"device_model"`
	DeviceType         DeviceType              `json:"device_type"`
	SubscriptionStatus SubscriptionStatus      `json:"subscription_status"`
	CoverageStatus     DashboardCoverageStatus `json:"coverage_status"`
	PlanNameFR         *string                 `json:"plan_name_fr,omitempty"`
	PlanNameEN         *string                 `json:"plan_name_en,omitempty"`
	PartnerStoreName   *string                 `json:"partner_store_name,omitempty"`
	FollowUp           *OperationalFollowUp    `json:"follow_up,omitempty"`
	Notes              []OperationalNote       `json:"notes"`
}

type EmployeeRepairDetail struct {
	Repair           RepairBooking        `json:"repair"`
	ClientID         *uuid.UUID           `json:"client_id,omitempty"`
	ClientEmail      *string              `json:"client_email,omitempty"`
	PartnerStoreName *string              `json:"partner_store_name,omitempty"`
	FollowUp         *OperationalFollowUp `json:"follow_up,omitempty"`
	Notes            []OperationalNote    `json:"notes"`
}

type EmployeeTaskItem struct {
	ID               string                `json:"id"`
	EntityType       OperationalEntityType `json:"entity_type"`
	EntityID         uuid.UUID             `json:"entity_id"`
	Title            string                `json:"title"`
	Description      string                `json:"description"`
	Reason           string                `json:"reason"`
	Priority         string                `json:"priority"`
	ClientName       string                `json:"client_name"`
	ClientEmail      *string               `json:"client_email,omitempty"`
	ClientPhone      *string               `json:"client_phone,omitempty"`
	PartnerStoreName *string               `json:"partner_store_name,omitempty"`
	Status           string                `json:"status"`
	FollowUpStatus   *FollowUpStatus       `json:"follow_up_status,omitempty"`
	NextAction       *string               `json:"next_action,omitempty"`
	LastContactAt    *time.Time            `json:"last_contact_at,omitempty"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

type EmployeeClientDetail struct {
	ID               uuid.UUID                      `json:"id"`
	FullName         string                         `json:"full_name"`
	Email            string                         `json:"email"`
	Phone            *string                        `json:"phone,omitempty"`
	PartnerStoreName *string                        `json:"partner_store_name,omitempty"`
	Devices          []EmployeeClientDeviceCoverage `json:"devices"`
	PaymentFollowUps []EmployeePaymentFollowUpItem  `json:"payment_follow_ups"`
	Claims           []EmployeeClaimDetail          `json:"claims"`
	Repairs          []EmployeeRepairDetail         `json:"repairs"`
	FollowUp         *OperationalFollowUp           `json:"follow_up,omitempty"`
	Notes            []OperationalNote              `json:"notes"`
}

type EmployeeDashboardOverview struct {
	Metrics          EmployeeOverviewMetrics       `json:"metrics"`
	PaymentFollowUps []EmployeePaymentFollowUpItem `json:"payment_follow_ups"`
	PendingClaims    []EmployeeClaimDetail         `json:"pending_claims"`
	ActiveRepairs    []EmployeeRepairDetail        `json:"active_repairs"`
	UrgentTasks      []EmployeeTaskItem            `json:"urgent_tasks"`
}

type PartnerDashboardOverview struct {
	Profile         *PartnerProfile         `json:"profile,omitempty"`
	ReferralLink    string                  `json:"referral_link"`
	ReferralMetrics *PartnerReferralMetrics `json:"referral_metrics,omitempty"`
	PlanBreakdown   []PartnerPlanBreakdown  `json:"plan_breakdown"`
	RecentClients   []PartnerClient         `json:"recent_clients"`
}

// AdminCustomer is a read-only view combining user + subscription + device data.
type AdminCustomerSubscription struct {
	ID                 string     `json:"id"`
	PlanID             string     `json:"plan_id"`
	PlanNameFR         *string    `json:"plan_name_fr,omitempty"`
	PlanNameEN         *string    `json:"plan_name_en,omitempty"`
	Status             string     `json:"status"`
	BillingCycle       string     `json:"billing_cycle"`
	DeviceID           string     `json:"device_id"`
	DeviceBrand        *string    `json:"device_brand,omitempty"`
	DeviceModel        *string    `json:"device_model,omitempty"`
	DeviceType         *string    `json:"device_type,omitempty"`
	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type AdminCustomer struct {
	ID                       string                      `json:"id"`
	FullName                 string                      `json:"full_name"`
	Phone                    *string                     `json:"phone,omitempty"`
	Email                    string                      `json:"email"`
	PartnerStoreName         *string                     `json:"partner_store_name,omitempty"`
	PartnerReferralCode      *string                     `json:"partner_referral_code,omitempty"`
	PartnerAttributionSource *string                     `json:"partner_attribution_source,omitempty"`
	PartnerAttributedAt      *time.Time                  `json:"partner_attributed_at,omitempty"`
	DeviceCount              int                         `json:"device_count"`
	ActiveSubscriptionCount  int                         `json:"active_subscription_count"`
	TotalSubscriptionCount   int                         `json:"total_subscription_count"`
	Subscriptions            []AdminCustomerSubscription `json:"subscriptions"`
}

type AdminEmployeeWorkloadSummary struct {
	ClientsFollowedCount int        `json:"clients_followed_count"`
	ActiveClaimsCount    int        `json:"active_claims_count"`
	ActiveRepairsCount   int        `json:"active_repairs_count"`
	OpenFollowUpsCount   int        `json:"open_follow_ups_count"`
	LastActivityAt       *time.Time `json:"last_activity_at,omitempty"`
	LastLoginAt          *time.Time `json:"last_login_at,omitempty"`
}

type AdminEmployeeListItem struct {
	ID              string                       `json:"id"`
	BetterAuthID    *string                      `json:"better_auth_id,omitempty"`
	FullName        string                       `json:"full_name"`
	Email           string                       `json:"email"`
	Phone           *string                      `json:"phone,omitempty"`
	Role            string                       `json:"role"`
	Status          EmployeeAccountStatus        `json:"status"`
	SuspendedReason *string                      `json:"suspended_reason,omitempty"`
	JoinedAt        time.Time                    `json:"joined_at"`
	Workload        AdminEmployeeWorkloadSummary `json:"workload"`
}

type AdminEmployeeActivityItem struct {
	Kind        string                `json:"kind"`
	EntityType  OperationalEntityType `json:"entity_type"`
	EntityID    string                `json:"entity_id"`
	Description string                `json:"description"`
	OccurredAt  time.Time             `json:"occurred_at"`
}

type AdminEmployeeDetail struct {
	ID                string                       `json:"id"`
	BetterAuthID      *string                      `json:"better_auth_id,omitempty"`
	FullName          string                       `json:"full_name"`
	Email             string                       `json:"email"`
	Phone             *string                      `json:"phone,omitempty"`
	Role              string                       `json:"role"`
	Status            EmployeeAccountStatus        `json:"status"`
	SuspendedReason   *string                      `json:"suspended_reason,omitempty"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
	WorkspaceAccess   bool                         `json:"workspace_access"`
	PermissionSummary []string                     `json:"permission_summary"`
	Workload          AdminEmployeeWorkloadSummary `json:"workload"`
	RecentActivity    []AdminEmployeeActivityItem  `json:"recent_activity"`
}

// AdminPayment is a read-only view combining payment + user + plan data.
type AdminPayment struct {
	ID            string     `json:"id"`
	CustomerName  string     `json:"customer_name"`
	PlanNameFR    *string    `json:"plan_name_fr,omitempty"`
	PlanNameEN    *string    `json:"plan_name_en,omitempty"`
	AmountMinor   int        `json:"amount_minor"`
	Market        MarketCode `json:"market"`
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
	RepairAmountMinor       *int       `json:"repair_amount_minor,omitempty"`
	Market                  MarketCode `json:"market"`
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
	AmountMinor     int             `json:"amount_minor"`
	Market          MarketCode      `json:"market"`
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
	ID                   uuid.UUID  `json:"id"`
	OrgID                uuid.UUID  `json:"org_id"`
	UserID               uuid.UUID  `json:"user_id"`
	StoreName            string     `json:"store_name"`
	City                 string     `json:"city"`
	BusinessLocation     string     `json:"business_location"`
	ReferralCode         string     `json:"referral_code"`
	CommissionPercentage float64    `json:"commission_percentage"`
	CommercialID         *uuid.UUID `json:"commercial_id,omitempty"`
	AcquisitionSource    string     `json:"acquisition_source"`
	Status               string     `json:"status"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// CommercialProfile represents a SafePhone commercial field agent.
type CommercialProfile struct {
	ID                   uuid.UUID `json:"id"`
	OrgID                uuid.UUID `json:"org_id"`
	UserID               uuid.UUID `json:"user_id"`
	ReferralCode         string    `json:"referral_code"`
	Status               string    `json:"status"`
	CommissionPercentage float64   `json:"commission_percentage"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// CommercialPartner is the commercial-facing view of a partner they acquired.
type CommercialPartner struct {
	ID                    string     `json:"id"`
	StoreName             string     `json:"store_name"`
	OwnerName             string     `json:"owner_name"`
	OwnerEmail            string     `json:"owner_email"`
	Phone                 *string    `json:"phone,omitempty"`
	City                  string     `json:"city"`
	BusinessLocation      string     `json:"business_location"`
	Status                string     `json:"status"`
	ApplicationStatus     *string    `json:"application_status,omitempty"`
	ApprovalDate          *time.Time `json:"approval_date,omitempty"`
	PartnerCommissionRate float64    `json:"partner_commission_percentage"`
	ClientsCount          int        `json:"clients_count"`
	ActiveClients         int        `json:"active_clients"`
	FirstPaymentStatus    *string    `json:"first_payment_status,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

// CommercialCommission records a one-time commercial commission for a partner's first client payment.
type CommercialCommission struct {
	ID                   uuid.UUID  `json:"id"`
	OrgID                uuid.UUID  `json:"org_id"`
	CommercialID         uuid.UUID  `json:"commercial_id"`
	PartnerID            uuid.UUID  `json:"partner_id"`
	PartnerClientID      *uuid.UUID `json:"partner_client_id,omitempty"`
	ClientUserID         *uuid.UUID `json:"client_user_id,omitempty"`
	PaymentID            *uuid.UUID `json:"payment_id,omitempty"`
	PlanID               *uuid.UUID `json:"plan_id,omitempty"`
	BaseAmountXOF        int        `json:"base_amount_xof"`
	CommissionPercentage float64    `json:"commission_percentage"`
	CommissionAmountXOF  int        `json:"commission_amount_xof"`
	Status               string     `json:"status"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// CommercialCommissionView is a read model for commercial/admin commission lists.
type CommercialCommissionView struct {
	ID                   string     `json:"id"`
	CommercialID         string     `json:"commercial_id"`
	CommercialName       string     `json:"commercial_name"`
	PartnerID            string     `json:"partner_id"`
	PartnerStoreName     string     `json:"partner_store_name"`
	PartnerClientID      *string    `json:"partner_client_id,omitempty"`
	ClientUserID         *string    `json:"client_user_id,omitempty"`
	ClientName           string     `json:"client_name"`
	PaymentID            *string    `json:"payment_id,omitempty"`
	PlanID               *string    `json:"plan_id,omitempty"`
	PlanNameFR           *string    `json:"plan_name_fr,omitempty"`
	PlanNameEN           *string    `json:"plan_name_en,omitempty"`
	BaseAmountXOF        int        `json:"base_amount_xof"`
	CommissionPercentage float64    `json:"commission_percentage"`
	CommissionAmountXOF  int        `json:"commission_amount_xof"`
	Status               string     `json:"status"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

// CommercialActivityReport stores field activity proof submitted by a commercial.
type CommercialActivityReport struct {
	ID               uuid.UUID  `json:"id"`
	OrgID            uuid.UUID  `json:"org_id"`
	CommercialID     uuid.UUID  `json:"commercial_id"`
	PartnerID        *uuid.UUID `json:"partner_id,omitempty"`
	ProspectName     *string    `json:"prospect_name,omitempty"`
	ActivityType     string     `json:"activity_type"`
	PhotoURL         string     `json:"photo_url"`
	PhotoStoragePath string     `json:"-"`
	PhotoContentType string     `json:"-"`
	Comment          string     `json:"comment"`
	City             *string    `json:"city,omitempty"`
	Location         *string    `json:"location,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// CommercialActivityReportView decorates activity reports with names/statuses.
type CommercialActivityReportView struct {
	ID               string    `json:"id"`
	CommercialID     string    `json:"commercial_id"`
	CommercialName   string    `json:"commercial_name"`
	PartnerID        *string   `json:"partner_id,omitempty"`
	PartnerStoreName *string   `json:"partner_store_name,omitempty"`
	PartnerStatus    *string   `json:"partner_status,omitempty"`
	ProspectName     *string   `json:"prospect_name,omitempty"`
	ActivityType     string    `json:"activity_type"`
	PhotoURL         string    `json:"photo_url"`
	Comment          string    `json:"comment"`
	City             *string   `json:"city,omitempty"`
	Location         *string   `json:"location,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// CommercialDashboardOverview combines commercial metrics and recent activity.
type CommercialDashboardOverview struct {
	Profile                    CommercialProfile              `json:"profile"`
	ReferralLink               string                         `json:"referral_link"`
	PartnersBrought            int                            `json:"partners_brought"`
	PendingPartnerApplications int                            `json:"pending_partner_applications"`
	ApprovedPartners           int                            `json:"approved_partners"`
	ActivePartners             int                            `json:"active_partners"`
	FirstClientConversions     int                            `json:"first_client_conversions"`
	CommissionEarnedXOF        int                            `json:"commission_earned_xof"`
	CommissionPendingXOF       int                            `json:"commission_pending_xof"`
	RecentPartners             []CommercialPartner            `json:"recent_partners"`
	RecentReports              []CommercialActivityReportView `json:"recent_reports"`
	RecentCommissions          []CommercialCommissionView     `json:"recent_commissions"`
}

// PartnerClient represents a client invited by a partner.
type PartnerClient struct {
	ID                     uuid.UUID  `json:"id"`
	OrgID                  uuid.UUID  `json:"org_id"`
	PartnerID              uuid.UUID  `json:"partner_id"`
	LinkedUserID           *uuid.UUID `json:"linked_user_id,omitempty"`
	ClientName             string     `json:"client_name"`
	ClientPhone            *string    `json:"client_phone,omitempty"`
	PlanID                 *uuid.UUID `json:"plan_id,omitempty"`
	Status                 string     `json:"status"`
	AttributionSource      string     `json:"attribution_source"`
	ReferralCode           *string    `json:"referral_code,omitempty"`
	ReferralMedium         string     `json:"referral_medium"`
	AttributedAt           *time.Time `json:"attributed_at,omitempty"`
	InvitationToken        string     `json:"-"`
	InvitationURL          string     `json:"invitation_url,omitempty"`
	InvitationExpiresAt    *time.Time `json:"invitation_expires_at,omitempty"`
	InvitationClaimedAt    *time.Time `json:"invitation_claimed_at,omitempty"`
	HasGeneratedCommission bool       `json:"has_generated_commission"`
	CommissionAmountXOF    *int       `json:"commission_amount_xof,omitempty"`
	CommissionStatus       *string    `json:"commission_status,omitempty"`
	CommissionPercentage   *float64   `json:"commission_percentage,omitempty"`
	CommissionCreatedAt    *time.Time `json:"commission_created_at,omitempty"`
	InvitedAt              time.Time  `json:"invited_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
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

// PartnerReferralDetails is the public referral context for reusable partner links.
type PartnerReferralDetails struct {
	PartnerID        uuid.UUID `json:"partner_id"`
	PartnerStoreName string    `json:"partner_store_name"`
	PartnerCity      string    `json:"partner_city"`
	ReferralCode     string    `json:"referral_code"`
	ReferralLink     string    `json:"referral_link,omitempty"`
	Status           string    `json:"status"`
}

// PartnerReferralVisit represents a public referral landing event.
type PartnerReferralVisit struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"org_id"`
	PartnerID    uuid.UUID `json:"partner_id"`
	ReferralCode string    `json:"referral_code"`
	VisitorToken string    `json:"visitor_token"`
	SourceMedium string    `json:"source_medium"`
	VisitedAt    time.Time `json:"visited_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// PartnerReferralVisitResult is returned after tracking a referral visit.
type PartnerReferralVisitResult struct {
	Referral     *PartnerReferralDetails `json:"referral,omitempty"`
	VisitorToken string                  `json:"visitor_token"`
	SourceMedium string                  `json:"source_medium"`
	VisitedAt    time.Time               `json:"visited_at"`
}

// PartnerReferralMetrics aggregates reusable-link acquisition performance.
type PartnerReferralMetrics struct {
	TotalVisits         int     `json:"total_visits"`
	QRVisits            int     `json:"qr_visits"`
	ShareVisits         int     `json:"share_visits"`
	TotalSignups        int     `json:"total_signups"`
	PaymentPendingCount int     `json:"payment_pending_count"`
	ActiveClients       int     `json:"active_clients"`
	ConversionRate      float64 `json:"conversion_rate"`
}

// PartnerPlanBreakdown summarizes referred customers by chosen plan.
type PartnerPlanBreakdown struct {
	PlanID     *string `json:"plan_id,omitempty"`
	PlanNameFR *string `json:"plan_name_fr,omitempty"`
	PlanNameEN *string `json:"plan_name_en,omitempty"`
	Count      int     `json:"count"`
}

// PartnerCommission represents an earned commission record.
type PartnerCommission struct {
	ID                   uuid.UUID  `json:"id"`
	OrgID                uuid.UUID  `json:"org_id"`
	PartnerID            uuid.UUID  `json:"partner_id"`
	PartnerClientID      *uuid.UUID `json:"partner_client_id,omitempty"`
	ClientUserID         *uuid.UUID `json:"client_user_id,omitempty"`
	PaymentID            *uuid.UUID `json:"payment_id,omitempty"`
	PlanID               *uuid.UUID `json:"plan_id,omitempty"`
	BaseAmountXOF        int        `json:"base_amount_xof"`
	CommissionPercentage float64    `json:"commission_percentage"`
	CommissionAmountXOF  int        `json:"commission_amount_xof"`
	Status               string     `json:"status"`
	PayoutMethod         *string    `json:"payout_method,omitempty"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// PartnerProfile combines partner identity with aggregated stats.
type PartnerProfile struct {
	ID                       uuid.UUID `json:"id"`
	StoreName                string    `json:"store_name"`
	City                     string    `json:"city"`
	BusinessLocation         string    `json:"business_location"`
	ReferralCode             string    `json:"referral_code"`
	CommissionPercentage     float64   `json:"commission_percentage"`
	Status                   string    `json:"status"`
	TotalClients             int       `json:"total_clients"`
	ActiveClients            int       `json:"active_clients"`
	PlansPurchased           int       `json:"plans_purchased"`
	TotalCommissionEarnedXOF int       `json:"total_commission_earned_xof"`
	TotalCommissionOwedXOF   int       `json:"total_commission_owed_xof"`
	TotalCommissionPaidXOF   int       `json:"total_commission_paid_xof"`
}

// PartnerSale is a read-only view of a commission with payment details.
type PartnerSale struct {
	ID                   string     `json:"id"`
	PartnerClientID      *string    `json:"partner_client_id,omitempty"`
	ClientUserID         *string    `json:"client_user_id,omitempty"`
	PaymentID            *string    `json:"payment_id,omitempty"`
	PlanID               *string    `json:"plan_id,omitempty"`
	CustomerName         string     `json:"customer_name"`
	PlanNameFR           *string    `json:"plan_name_fr,omitempty"`
	PlanNameEN           *string    `json:"plan_name_en,omitempty"`
	BaseAmountXOF        int        `json:"base_amount_xof"`
	CommissionPercentage float64    `json:"commission_percentage"`
	CommissionAmountXOF  int        `json:"commission_amount_xof"`
	Status               string     `json:"status"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
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
	ID                       string  `json:"id"`
	StoreName                string  `json:"store_name"`
	OwnerName                string  `json:"owner_name"`
	OwnerEmail               string  `json:"owner_email"`
	OwnerPhone               *string `json:"owner_phone,omitempty"`
	City                     string  `json:"city"`
	BusinessLocation         string  `json:"business_location"`
	ReferralCode             string  `json:"referral_code"`
	CommissionPercentage     float64 `json:"commission_percentage"`
	CommercialID             *string `json:"commercial_id,omitempty"`
	CommercialName           *string `json:"commercial_name,omitempty"`
	CommercialEmail          *string `json:"commercial_email,omitempty"`
	AcquisitionSource        string  `json:"acquisition_source"`
	ClientsCount             int     `json:"clients_count"`
	ActiveClients            int     `json:"active_clients"`
	FirstPaymentStatus       *string `json:"first_payment_status,omitempty"`
	ReferralVisits           int     `json:"referral_visits"`
	QRReferralVisits         int     `json:"qr_referral_visits"`
	ReferralSignups          int     `json:"referral_signups"`
	ReferralActivations      int     `json:"referral_activations"`
	ReferralConversionRate   float64 `json:"referral_conversion_rate"`
	TotalCommissionEarnedXOF int     `json:"total_commission_earned_xof"`
	TotalCommissionOwedXOF   int     `json:"total_commission_owed_xof"`
	TotalCommissionPaidXOF   int     `json:"total_commission_paid_xof"`
	Status                   string  `json:"status"`
	JoinedAt                 string  `json:"joined_at"`
}

// AdminPartnerReferral is a detailed admin view of customers attributed to a partner.
type AdminPartnerReferral struct {
	PartnerClientID        string     `json:"partner_client_id"`
	ClientUserID           *string    `json:"client_user_id,omitempty"`
	CustomerName           string     `json:"customer_name"`
	CustomerEmail          *string    `json:"customer_email,omitempty"`
	CustomerPhone          *string    `json:"customer_phone,omitempty"`
	AttributionSource      string     `json:"attribution_source"`
	ReferralCode           *string    `json:"referral_code,omitempty"`
	ReferralMedium         string     `json:"referral_medium"`
	AttributedAt           *time.Time `json:"attributed_at,omitempty"`
	PlanID                 *string    `json:"plan_id,omitempty"`
	PlanNameFR             *string    `json:"plan_name_fr,omitempty"`
	PlanNameEN             *string    `json:"plan_name_en,omitempty"`
	ClientStatus           string     `json:"client_status"`
	SubscriptionStatus     *string    `json:"subscription_status,omitempty"`
	PaymentStatus          *string    `json:"payment_status,omitempty"`
	HasGeneratedCommission bool       `json:"has_generated_commission"`
	CommissionAmountXOF    *int       `json:"commission_amount_xof,omitempty"`
	CommissionStatus       *string    `json:"commission_status,omitempty"`
}

// AdminPartnerCommission is a read-only admin view of a partner commission line item.
type AdminPartnerCommission struct {
	ID                   string     `json:"id"`
	PartnerClientID      *string    `json:"partner_client_id,omitempty"`
	ClientUserID         *string    `json:"client_user_id,omitempty"`
	PaymentID            *string    `json:"payment_id,omitempty"`
	PlanID               *string    `json:"plan_id,omitempty"`
	CustomerName         string     `json:"customer_name"`
	PlanNameFR           *string    `json:"plan_name_fr,omitempty"`
	PlanNameEN           *string    `json:"plan_name_en,omitempty"`
	BaseAmountXOF        int        `json:"base_amount_xof"`
	CommissionPercentage float64    `json:"commission_percentage"`
	CommissionAmountXOF  int        `json:"commission_amount_xof"`
	Status               string     `json:"status"`
	PaidAt               *time.Time `json:"paid_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

// AdminCommercialListItem summarizes commercial performance for admin.
type AdminCommercialListItem struct {
	ID                   string     `json:"id"`
	UserID               string     `json:"user_id"`
	Name                 string     `json:"name"`
	Email                string     `json:"email"`
	Phone                *string    `json:"phone,omitempty"`
	Status               string     `json:"status"`
	ReferralCode         string     `json:"referral_code"`
	CommissionPercentage float64    `json:"commission_percentage"`
	PartnersBrought      int        `json:"partners_brought"`
	ApprovedPartners     int        `json:"approved_partners"`
	PendingPartners      int        `json:"pending_partners"`
	CommissionEarnedXOF  int        `json:"commission_earned_xof"`
	LastActivityDate     *time.Time `json:"last_activity_date,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

// AdminCommercialDetail contains one commercial plus their partners, reports, and commissions.
type AdminCommercialDetail struct {
	Commercial  AdminCommercialListItem        `json:"commercial"`
	Partners    []CommercialPartner            `json:"partners"`
	Reports     []CommercialActivityReportView `json:"reports"`
	Commissions []CommercialCommissionView     `json:"commissions"`
}
