// Package service — Stripe US flow.
//
// Lives alongside the existing DEXPAY-based PaymentService. Keeping the
// two flows separate avoids tangling two state machines: DEXPAY's
// atomic device+subscription creation vs Stripe's customer/checkout/
// invoice/webhook lifecycle.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	stripego "github.com/stripe/stripe-go/v82"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/config"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
	"github.com/cherif-safephone/safephone-backend/internal/repository"
	"github.com/cherif-safephone/safephone-backend/internal/stripe"
)

// StripeService handles US-market checkout and webhook processing.
type StripeService struct {
	client        *stripe.Client
	cfg           *config.Config
	users         *repository.UserRepository
	subs          *repository.SubscriptionRepository
	plans         *repository.PlanRepository
	devices       *repository.DeviceRepository
	payments      *repository.PaymentRepository
	subDevices    *repository.SubscriptionDevicesRepository
	webhookEvents *repository.WebhookEventRepository
}

// NewStripeService creates a new Stripe service. client may be nil when
// Stripe is not configured — methods return a clear error in that case.
func NewStripeService(
	client *stripe.Client,
	cfg *config.Config,
	users *repository.UserRepository,
	subs *repository.SubscriptionRepository,
	plans *repository.PlanRepository,
	devices *repository.DeviceRepository,
	payments *repository.PaymentRepository,
	subDevices *repository.SubscriptionDevicesRepository,
	webhookEvents *repository.WebhookEventRepository,
) *StripeService {
	return &StripeService{
		client:        client,
		cfg:           cfg,
		users:         users,
		subs:          subs,
		plans:         plans,
		devices:       devices,
		payments:      payments,
		subDevices:    subDevices,
		webhookEvents: webhookEvents,
	}
}

// Enabled reports whether the service can process requests.
func (s *StripeService) Enabled() bool {
	return s != nil && s.client != nil
}

// StripeCheckoutResult is what the handler returns to the frontend.
type StripeCheckoutResult struct {
	CheckoutURL string `json:"checkout_url"`
	SessionID   string `json:"session_id"`
}

// CreateCheckout starts a US Stripe Checkout session for the
// authenticated user and the requested plan slug. Frontend never sends
// a price — only a plan slug — and the backend resolves the Stripe price
// ID from env config.
func (s *StripeService) CreateCheckout(
	ctx context.Context,
	ac *auth.AuthContext,
	planSlug string,
) (*StripeCheckoutResult, *domain.AppError) {
	if !s.Enabled() {
		return nil, domain.ServiceUnavailable("Stripe payments are not configured")
	}

	priceID := s.cfg.StripePriceIDForPlan(planSlug)
	if priceID == "" {
		return nil, domain.BadRequest("unknown plan")
	}

	plan, err := s.plans.GetBySlug(ctx, planSlug)
	if err != nil {
		return nil, domain.Internal("failed to load plan")
	}
	if plan == nil {
		return nil, domain.BadRequest("plan not found")
	}

	user, err := s.users.GetByID(ctx, ac.UserID)
	if err != nil {
		return nil, domain.Internal("failed to load user")
	}
	if user == nil {
		return nil, domain.Unauthorized("user not found")
	}

	customerID, err := s.users.GetStripeCustomerID(ctx, user.ID)
	if err != nil {
		return nil, domain.Internal("failed to load Stripe customer")
	}
	if customerID == "" {
		customerID, err = s.client.CreateCustomer(ctx, stripe.CreateCustomerParams{
			Email:  user.Email,
			Name:   user.FullName,
			UserID: user.ID.String(),
		})
		if err != nil {
			slog.Error("stripe create customer failed", "error", err, "user_id", user.ID)
			return nil, domain.Internal("failed to create Stripe customer")
		}
		if err := s.users.SetStripeCustomerID(ctx, user.ID, customerID); err != nil {
			slog.Error("stripe customer persist failed", "error", err, "user_id", user.ID, "customer_id", customerID)
			return nil, domain.Internal("failed to persist Stripe customer")
		}
	}

	metadata := map[string]string{
		"safephone_user_id":   user.ID.String(),
		"safephone_org_id":    user.OrgID.String(),
		"safephone_plan_id":   plan.ID.String(),
		"safephone_plan_slug": plan.Slug,
		"safephone_market":    "US",
	}

	sess, err := s.client.CreateCheckoutSession(ctx, stripe.CreateCheckoutSessionParams{
		CustomerID: customerID,
		PriceID:    priceID,
		Metadata:   metadata,
		// UnixNano avoids the 1s collision window of Unix() — a user
		// double-clicking "Subscribe" within the same second would otherwise
		// share an idempotency key and accidentally hit Stripe's idempotency
		// replay path (returning the first session even if intent changed).
		IdempotencyKey: fmt.Sprintf("checkout-%s-%s-%d", user.ID, plan.ID, time.Now().UnixNano()),
	})
	if err != nil {
		slog.Error("stripe checkout create failed", "error", err, "user_id", user.ID, "plan", plan.Slug)
		return nil, domain.Internal("failed to create Stripe checkout session")
	}

	return &StripeCheckoutResult{CheckoutURL: sess.URL, SessionID: sess.ID}, nil
}

// RegisterDeviceParams is the input for US post-checkout device registration.
// Plans v2 added the verification fields: 2 photo URLs + 1 video URL are
// required so an admin can review the device condition before the
// subscription transitions to active.
type RegisterDeviceParams struct {
	Brand        string
	Model        string
	IMEI         string
	DeviceType   string
	SerialNumber string
	// Verification media. Empty values are allowed at the service layer
	// (the handler enforces the 2+1 minimum) so older callers that
	// pre-date plans v2 don't break.
	Photos []string
	Video  string
}

// RegisterDeviceResult returns the new device + the subscription it was attached to.
type RegisterDeviceResult struct {
	Device       *domain.Device       `json:"device"`
	Subscription *domain.Subscription `json:"subscription"`
}

// RegisterDevice creates a device for the authenticated US user and
// attaches it to their most-recent US subscription that has no device
// yet. Idempotent — calling twice doesn't double-attach.
func (s *StripeService) RegisterDevice(
	ctx context.Context,
	ac *auth.AuthContext,
	p RegisterDeviceParams,
) (*RegisterDeviceResult, *domain.AppError) {
	if p.Model == "" {
		return nil, domain.BadRequest("model is required")
	}
	// Brand is required only for smartphones; non-phones (TV, console,
	// tablet, computer) carry the full name in Model and send no brand.
	if domain.NormalizeDeviceType(p.DeviceType) == domain.DeviceTypeSmartphone && p.Brand == "" {
		return nil, domain.BadRequest("brand is required for smartphones")
	}
	if p.IMEI != "" && !domain.IsValidIMEI(p.IMEI) {
		return nil, domain.BadRequest("IMEI is not valid (must be 15 digits with a valid checksum)")
	}

	user, err := s.users.GetByID(ctx, ac.UserID)
	if err != nil {
		return nil, domain.Internal("failed to load user")
	}
	if user == nil {
		return nil, domain.Unauthorized("user not found")
	}

	subscription, err := s.subs.FindUSPendingSubscriptionWithoutDevice(ctx, user.ID)
	if err != nil {
		return nil, domain.Internal("failed to look up subscription")
	}
	if subscription == nil {
		return nil, domain.NotFound("US subscription awaiting device registration")
	}

	deviceType := domain.NormalizeDeviceType(p.DeviceType)

	// Enforce the subscription plan's device coverage + per-type cap before
	// attaching. The plan covers a fixed number of each device type.
	plan, err := s.plans.GetByID(ctx, subscription.PlanID)
	if err != nil {
		return nil, domain.Internal("failed to load plan")
	}
	if plan != nil {
		if !domain.PlanAllowsDeviceType(plan, deviceType) {
			return nil, domain.BadRequest("this plan does not cover this device type")
		}
		counts, err := s.subDevices.CountByType(ctx, subscription.ID)
		if err != nil {
			return nil, domain.Internal("failed to count attached devices")
		}
		if counts[deviceType] >= plan.MaxForDeviceType(deviceType) {
			return nil, domain.BadRequest("device limit reached for this device type on this plan")
		}
	}

	device := &domain.Device{
		OrgID:      user.OrgID,
		UserID:     user.ID,
		DeviceType: deviceType,
		Brand:      p.Brand,
		Model:      p.Model,
		IMEI:       p.IMEI,
		Metadata:   domain.DeviceMetadata{SerialNumber: p.SerialNumber},
		// Devices registered under plans v2 stay pending until admin
		// approves the verification media. Legacy callers that omit
		// photos/video skip the gate (the field stays empty).
		Status: domain.DeviceStatusPending,
		Market: domain.MarketUS,
	}
	if err := s.devices.Create(ctx, device); err != nil {
		slog.Error("us register device: create device failed", "error", err, "user_id", user.ID)
		return nil, domain.Internal("failed to register device")
	}

	// Persist the verification media when supplied. The handler validated
	// the 2-photo + 1-video minimum upstream; we re-check here as defense
	// in depth so a future internal caller can't bypass the gate.
	if len(p.Photos) > 0 || p.Video != "" {
		if appErr := validateVerificationMedia(deviceType, p.Photos, p.Video); appErr != nil {
			return nil, appErr
		}
		if err := s.devices.SetVerificationMedia(ctx, device.ID, p.Photos, p.Video); err != nil {
			slog.Error("us register device: store verification media failed", "error", err, "device_id", device.ID)
			return nil, domain.Internal("failed to record verification media")
		}
	}

	if err := s.subs.AttachDevice(ctx, subscription.ID, device.ID); err != nil {
		slog.Error("us register device: attach failed", "error", err, "subscription_id", subscription.ID, "device_id", device.ID)
		return nil, domain.Internal("failed to attach device to subscription")
	}
	if err := s.subDevices.Attach(ctx, subscription.ID, device.ID); err != nil {
		slog.Error("us register device: attach to subscription_devices failed", "error", err, "subscription_id", subscription.ID, "device_id", device.ID)
		return nil, domain.Internal("failed to attach device to subscription")
	}

	return &RegisterDeviceResult{Device: device, Subscription: subscription}, nil
}

// HandleEvent dispatches a verified Stripe webhook to the right
// state-transition handler. Events are dedup'd via the webhook_events
// idempotency_key; unknown event types are recorded and accepted (200)
// so Stripe doesn't retry them.
func (s *StripeService) HandleEvent(ctx context.Context, event stripego.Event, rawPayload []byte) *domain.AppError {
	idempotencyKey := "stripe:" + event.ID

	// Atomic insert-or-skip: if another concurrent webhook delivery already
	// claimed this event ID, CreateIfNew returns created=false and we ack
	// without re-processing. This replaces a previous Exists()+Create() pair
	// that had a TOCTOU window.
	created, err := s.webhookEvents.CreateIfNew(ctx, &domain.WebhookEvent{
		Provider:       "stripe",
		EventType:      string(event.Type),
		ProviderRef:    event.ID,
		IdempotencyKey: idempotencyKey,
		Payload:        rawPayload,
	})
	if err != nil {
		slog.Error("stripe webhook dedup insert failed", "error", err, "event_id", event.ID)
		return domain.Internal("failed to record webhook event")
	}
	if !created {
		slog.Info("stripe webhook duplicate ignored", "event_id", event.ID, "type", event.Type)
		return nil
	}

	switch event.Type {
	case "checkout.session.completed":
		if appErr := s.handleCheckoutSessionCompleted(ctx, event); appErr != nil {
			return appErr
		}
	case "invoice.paid", "invoice.payment_succeeded":
		if appErr := s.handleInvoicePaid(ctx, event); appErr != nil {
			return appErr
		}
	case "invoice.payment_failed":
		if appErr := s.handleInvoicePaymentFailed(ctx, event); appErr != nil {
			return appErr
		}
	case "customer.subscription.deleted":
		if appErr := s.handleSubscriptionDeleted(ctx, event); appErr != nil {
			return appErr
		}
	case "customer.subscription.created", "customer.subscription.updated":
		if appErr := s.handleSubscriptionUpsert(ctx, event); appErr != nil {
			return appErr
		}
	case "charge.dispute.created":
		slog.Warn("stripe charge dispute opened", "event_id", event.ID)
	default:
		slog.Debug("stripe webhook unhandled type", "type", event.Type, "event_id", event.ID)
	}

	return nil
}

func (s *StripeService) handleCheckoutSessionCompleted(ctx context.Context, event stripego.Event) *domain.AppError {
	var sess stripego.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		slog.Error("stripe decode checkout.session.completed failed", "error", err)
		return domain.BadRequest("invalid checkout.session payload")
	}

	meta := sess.Metadata
	userID, ok1 := parseUUIDMeta(meta, "safephone_user_id")
	orgID, ok2 := parseUUIDMeta(meta, "safephone_org_id")
	planID, ok3 := parseUUIDMeta(meta, "safephone_plan_id")
	if !ok1 || !ok2 || !ok3 {
		slog.Error("stripe checkout.session.completed missing metadata", "session_id", sess.ID)
		return domain.BadRequest("missing SafePhone metadata on checkout session")
	}
	if sess.Subscription == nil || sess.Subscription.ID == "" {
		slog.Warn("stripe checkout.session.completed has no subscription", "session_id", sess.ID)
		return nil
	}

	_, err := s.subs.CreateStripeSubscription(ctx, repository.CreateStripeSubscriptionParams{
		OrgID:                   orgID,
		UserID:                  userID,
		PlanID:                  planID,
		BillingCycle:            "monthly",
		Status:                  domain.SubscriptionStatusPending,
		StripeSubscriptionID:    sess.Subscription.ID,
		StripeCheckoutSessionID: sess.ID,
	})
	if err != nil {
		slog.Error("stripe create subscription row failed", "error", err, "session_id", sess.ID)
		return domain.Internal("failed to record Stripe subscription")
	}
	return nil
}

func (s *StripeService) handleInvoicePaid(ctx context.Context, event stripego.Event) *domain.AppError {
	var inv stripego.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return domain.BadRequest("invalid invoice payload")
	}
	subID := invoiceSubscriptionID(&inv)
	if subID == "" {
		return nil
	}
	// Pass nil periods on purpose: invoice.period_start/end describe the
	// invoiced line (which can be a zero-length range for the immediate
	// charge of a fresh subscription), not the recurring billing window.
	// The canonical period is owned by customer.subscription.created /
	// .updated; we'd overwrite it here otherwise. COALESCE in the UPDATE
	// preserves whatever those handlers set.
	// Plans v2: a paid invoice no longer flips the subscription straight to
	// active. The sub enters pending_verification first, then an admin
	// reviews the user's uploaded photos/videos and calls
	// SubscriptionService.ConfirmVerification to set status=active +
	// activated_at=now. Legacy claim flows that read status==active won't
	// be triggered until verification clears.
	err := s.subs.UpdateStripeSubscriptionState(
		ctx, subID,
		domain.SubscriptionStatusPendingVerification,
		nil, nil, nil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("stripe invoice.paid: no matching subscription yet", "stripe_sub", subID)
		return nil
	}
	if err != nil {
		slog.Error("stripe invoice.paid update failed", "error", err, "stripe_sub", subID)
		return domain.Internal("failed to activate subscription")
	}

	// Record the Stripe payment in our `payments` table so the user / admin
	// dashboards see this transaction alongside DEXPAY ones. Idempotency key
	// is "stripe:invoice:<inv.ID>" — Stripe retries the same invoice event
	// on transient failures, and we don't want duplicate rows.
	if appErr := s.recordStripePayment(ctx, &inv, subID); appErr != nil {
		// Logged inside; don't bubble — the subscription is already active,
		// the payment record is a secondary book-keeping concern. Stripe
		// will retry the webhook on a 5xx response and we'd risk
		// re-activating the sub.
		slog.Error("stripe invoice.paid: payment row not recorded",
			"stripe_sub", subID,
			"invoice_id", inv.ID,
			"error", appErr.Message)
	}
	return nil
}

// recordStripePayment inserts a Payment row for a paid Stripe invoice.
// Returns nil silently if a row with the same idempotency key already
// exists (Stripe webhook retries).
func (s *StripeService) recordStripePayment(
	ctx context.Context,
	inv *stripego.Invoice,
	stripeSubID string,
) *domain.AppError {
	idempotencyKey := fmt.Sprintf("stripe:invoice:%s", inv.ID)

	existing, err := s.payments.GetByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		return domain.Internal("failed to check payment dedup")
	}
	if existing != nil {
		return nil // already recorded — retry from Stripe
	}

	sub, err := s.subs.GetByStripeSubscriptionID(ctx, stripeSubID)
	if err != nil {
		return domain.Internal("failed to load subscription for payment record")
	}
	if sub == nil {
		return domain.Internal("subscription not found for stripe sub " + stripeSubID)
	}

	currency := "USD"
	if inv.Currency != "" {
		currency = strings.ToUpper(string(inv.Currency))
	}
	providerRef := inv.ID
	now := time.Now().UTC()
	market := sub.Market
	if market == "" {
		market = domain.MarketUS
	}

	payment := &domain.Payment{
		ID:             uuid.New(),
		OrgID:          sub.OrgID,
		UserID:         sub.UserID,
		PlanID:         sub.PlanID,
		SubscriptionID: sub.ID,
		AmountMinor:    int(inv.AmountPaid),
		Market:         market,
		Currency:       currency,
		Provider:       "stripe",
		Status:         domain.PaymentStatusCompleted,
		ProviderRef:    &providerRef,
		IdempotencyKey: &idempotencyKey,
		PaidAt:         &now,
	}
	if err := s.payments.Create(ctx, payment); err != nil {
		return domain.Internal("failed to insert payment row")
	}
	return nil
}

func (s *StripeService) handleInvoicePaymentFailed(ctx context.Context, event stripego.Event) *domain.AppError {
	var inv stripego.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return domain.BadRequest("invalid invoice payload")
	}
	subID := invoiceSubscriptionID(&inv)
	if subID == "" {
		return nil
	}
	err := s.subs.UpdateStripeSubscriptionState(
		ctx, subID,
		domain.SubscriptionStatusPastDue,
		nil, nil, nil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		slog.Error("stripe invoice.payment_failed update failed", "error", err, "stripe_sub", subID)
		return domain.Internal("failed to mark subscription past due")
	}
	return nil
}

// handleSubscriptionUpsert reads the canonical billing period directly
// from the Subscription object. In stripe-go 82 (API basil+) the
// current_period_start/end fields live on SubscriptionItem, not on the
// Subscription itself — reading them off the invoice gives line-item
// periods that can be equal/zero for the first paid invoice, which would
// otherwise make the subscription look already-expired.
func (s *StripeService) handleSubscriptionUpsert(ctx context.Context, event stripego.Event) *domain.AppError {
	var sub stripego.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return domain.BadRequest("invalid subscription payload")
	}
	if sub.ID == "" {
		return nil
	}

	start, end := subscriptionPeriod(&sub)
	status := mapStripeSubscriptionStatus(sub.Status)

	err := s.subs.UpdateStripeSubscriptionState(ctx, sub.ID, status, start, end, nil)
	if errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("stripe subscription upsert: no matching row yet", "stripe_sub", sub.ID)
		return nil
	}
	if err != nil {
		slog.Error("stripe subscription upsert failed", "error", err, "stripe_sub", sub.ID)
		return domain.Internal("failed to update subscription period")
	}
	return nil
}

func (s *StripeService) handleSubscriptionDeleted(ctx context.Context, event stripego.Event) *domain.AppError {
	var sub stripego.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return domain.BadRequest("invalid subscription payload")
	}
	now := time.Now().UTC()
	err := s.subs.UpdateStripeSubscriptionState(
		ctx, sub.ID,
		domain.SubscriptionStatusCancelled,
		nil, nil, &now,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		slog.Error("stripe subscription.deleted update failed", "error", err, "stripe_sub", sub.ID)
		return domain.Internal("failed to cancel subscription")
	}
	return nil
}

func parseUUIDMeta(meta map[string]string, key string) (uuid.UUID, bool) {
	raw, ok := meta[key]
	if !ok {
		return uuid.Nil, false
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return parsed, true
}

func invoiceSubscriptionID(inv *stripego.Invoice) string {
	if inv == nil {
		return ""
	}
	// Stripe v82 splits subscription onto parent.subscription_details.
	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil &&
		inv.Parent.SubscriptionDetails.Subscription != nil {
		return inv.Parent.SubscriptionDetails.Subscription.ID
	}
	return ""
}

// subscriptionPeriod returns the canonical billing period for a Stripe
// Subscription. As of API version 2025-08-27.basil, current_period_start
// and current_period_end live on each SubscriptionItem (subscriptions
// can have items with different cadences). We take the first item's
// period — sufficient for SafePhone's single-item plans.
func subscriptionPeriod(sub *stripego.Subscription) (*time.Time, *time.Time) {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 {
		return nil, nil
	}
	item := sub.Items.Data[0]
	if item == nil || item.CurrentPeriodStart == 0 || item.CurrentPeriodEnd == 0 {
		return nil, nil
	}
	start := time.Unix(item.CurrentPeriodStart, 0).UTC()
	end := time.Unix(item.CurrentPeriodEnd, 0).UTC()
	return &start, &end
}

// mapStripeSubscriptionStatus translates Stripe's subscription status
// vocabulary to SafePhone's. Stripe has more granular states (trialing,
// incomplete, incomplete_expired, paused) that all collapse onto our
// pending/active/past_due/cancelled enum.
func mapStripeSubscriptionStatus(s stripego.SubscriptionStatus) domain.SubscriptionStatus {
	switch s {
	case stripego.SubscriptionStatusActive, stripego.SubscriptionStatusTrialing:
		// Plans v2: payment OK from Stripe's view, but our sub is gated
		// behind admin verification of the device photos/videos. We map
		// Stripe's "active" to our pending_verification until that
		// review clears.
		return domain.SubscriptionStatusPendingVerification
	case stripego.SubscriptionStatusPastDue, stripego.SubscriptionStatusUnpaid:
		return domain.SubscriptionStatusPastDue
	case stripego.SubscriptionStatusCanceled,
		stripego.SubscriptionStatusIncompleteExpired:
		return domain.SubscriptionStatusCancelled
	case stripego.SubscriptionStatusIncomplete, stripego.SubscriptionStatusPaused:
		return domain.SubscriptionStatusPending
	default:
		return domain.SubscriptionStatusPending
	}
}
