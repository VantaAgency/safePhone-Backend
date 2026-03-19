package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/database"
	"github.com/cherif-safephone/safephone-backend/internal/dexpay"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

const (
	dexpayProviderName = "dexpay"
	dexpayCurrencyXOF  = "XOF"
	dexpayCountrySN    = "SN"
)

// CheckoutResult is returned by Create when initiating a payment.
type CheckoutResult struct {
	Payment      *domain.Payment      `json:"payment"`
	Device       *domain.Device       `json:"device"`
	Subscription *domain.Subscription `json:"subscription"`
	PaymentURL   string               `json:"payment_url,omitempty"`
}

// PaymentService handles payment business logic.
type PaymentService struct {
	repo             domain.PaymentRepository
	subRepo          domain.SubscriptionRepository
	planRepo         domain.PlanRepository
	userRepo         domain.UserRepository
	deviceRepo       domain.DeviceRepository
	partnerRepo      domain.PartnerRepository
	webhookRepo      domain.WebhookEventRepository
	dexpayClient     *dexpay.Client
	pool             *pgxpool.Pool
	frontendURL      string
	backendPublicURL string
	devMode          bool
}

// NewPaymentService creates a new payment service.
func NewPaymentService(
	repo domain.PaymentRepository,
	subRepo domain.SubscriptionRepository,
	planRepo domain.PlanRepository,
	userRepo domain.UserRepository,
	deviceRepo domain.DeviceRepository,
	partnerRepo domain.PartnerRepository,
	webhookRepo domain.WebhookEventRepository,
	dexpayClient *dexpay.Client,
	pool *pgxpool.Pool,
	frontendURL string,
	backendPublicURL string,
	devMode bool,
) *PaymentService {
	return &PaymentService{
		repo:             repo,
		subRepo:          subRepo,
		planRepo:         planRepo,
		userRepo:         userRepo,
		deviceRepo:       deviceRepo,
		partnerRepo:      partnerRepo,
		webhookRepo:      webhookRepo,
		dexpayClient:     dexpayClient,
		pool:             pool,
		frontendURL:      strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
		backendPublicURL: strings.TrimRight(strings.TrimSpace(backendPublicURL), "/"),
		devMode:          devMode,
	}
}

// Create atomically creates a device, subscription, and payment, then initiates checkout.
func (s *PaymentService) Create(ctx context.Context, ac *auth.AuthContext,
	brand, model, imei string,
	planID uuid.UUID, billingCycle string,
	idempotencyKey *string,
) (*CheckoutResult, *domain.AppError) {
	if s.dexpayClient == nil && !s.devMode {
		return nil, domain.PaymentGatewayError(fmt.Errorf("DEXPAY is not configured: missing validated backend credentials"))
	}

	if idempotencyKey != nil {
		existing, err := s.repo.GetByIdempotencyKey(ctx, *idempotencyKey)
		if err != nil {
			return nil, domain.InternalError(err)
		}
		if existing != nil {
			if existing.OrgID != ac.OrgID || existing.UserID != ac.UserID {
				return nil, domain.Conflict("idempotency key is already in use")
			}
			return s.buildCheckoutResult(ctx, existing)
		}
	}

	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if appErr := validatePlanAvailability(plan, s.devMode); appErr != nil {
		return nil, appErr
	}

	if imei != "" {
		existing, err := s.deviceRepo.GetByIMEI(ctx, imei)
		if err != nil {
			return nil, domain.InternalError(err)
		}
		if existing != nil {
			return nil, domain.Conflict("a device with this IMEI already exists")
		}
	}

	amount := plan.PriceMonthly
	if billingCycle == "annual" {
		amount = plan.PriceAnnual
	}

	var imeiPtr *string
	if imei != "" {
		imeiPtr = &imei
	}

	var device domain.Device
	var sub domain.Subscription
	paymentID := uuid.New()
	providerRef := buildCheckoutReference(paymentID)
	payment := domain.Payment{
		ID:             paymentID,
		OrgID:          ac.OrgID,
		UserID:         ac.UserID,
		PlanID:         planID,
		AmountXOF:      amount,
		Currency:       dexpayCurrencyXOF,
		Provider:       dexpayProviderName,
		Status:         domain.PaymentStatusPending,
		ProviderRef:    &providerRef,
		IdempotencyKey: idempotencyKey,
	}

	txErr := database.WithTransaction(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO devices (org_id, user_id, brand, model, imei, status)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, created_at, updated_at
		`, ac.OrgID, ac.UserID, brand, model, imeiPtr, domain.DeviceStatusPending,
		).Scan(&device.ID, &device.CreatedAt, &device.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert device: %w", err)
		}
		device.OrgID = ac.OrgID
		device.UserID = ac.UserID
		device.Brand = brand
		device.Model = model
		device.IMEI = imei
		device.Status = domain.DeviceStatusPending

		err = tx.QueryRow(ctx, `
			INSERT INTO subscriptions (org_id, user_id, device_id, plan_id, status, billing_cycle,
			       current_period_start, current_period_end)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, created_at, updated_at
		`, ac.OrgID, ac.UserID, device.ID, planID, domain.SubscriptionStatusPending, billingCycle,
			nil, nil,
		).Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert subscription: %w", err)
		}
		sub.OrgID = ac.OrgID
		sub.UserID = ac.UserID
		sub.DeviceID = device.ID
		sub.PlanID = planID
		sub.Status = domain.SubscriptionStatusPending
		sub.BillingCycle = billingCycle
		sub.CurrentPeriodStart = nil
		sub.CurrentPeriodEnd = nil

		payment.SubscriptionID = sub.ID
		err = tx.QueryRow(ctx, `
			INSERT INTO payments (
				id, org_id, user_id, plan_id, subscription_id, amount_xof, currency,
				provider, payment_method, status, provider_ref, idempotency_key
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING created_at, updated_at
		`, payment.ID, payment.OrgID, payment.UserID, payment.PlanID, payment.SubscriptionID, payment.AmountXOF, payment.Currency,
			payment.Provider, payment.PaymentMethod, payment.Status, payment.ProviderRef, payment.IdempotencyKey,
		).Scan(&payment.CreatedAt, &payment.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert payment: %w", err)
		}

		return nil
	})
	if txErr != nil {
		return nil, domain.InternalError(txErr)
	}

	if s.dexpayClient != nil {
		result, appErr := s.initiateDexpayCheckout(ctx, ac, &device, &sub, &payment, plan)
		if appErr != nil {
			return nil, appErr
		}
		s.syncPartnerClientStatus(ctx, ac.UserID, "payment_pending", &planID)
		return result, nil
	}

	if s.devMode {
		slog.Warn("DEXPAY disabled, using development payment fallback")
		nowTime := time.Now()
		payment.Status = domain.PaymentStatusCompleted
		payment.PaidAt = &nowTime
		payment.FailedAt = nil
		if err := s.repo.Update(ctx, &payment); err != nil {
			return nil, domain.InternalError(err)
		}
		s.syncPartnerClientStatus(ctx, ac.UserID, "payment_pending", &planID)
		if appErr := s.activateCoverage(ctx, &sub, &device); appErr != nil {
			return nil, appErr
		}
	}

	return &CheckoutResult{
		Payment:      &payment,
		Device:       &device,
		Subscription: &sub,
		PaymentURL:   valueOrEmpty(payment.PaymentURL),
	}, nil
}

// HandleWebhookEvent processes an incoming DEXPAY webhook event idempotently.
func (s *PaymentService) HandleWebhookEvent(ctx context.Context, event dexpay.WebhookEvent, rawPayload []byte) *domain.AppError {
	providerRef, extractErr := extractProviderRef(event)
	if extractErr != nil {
		return domain.BadRequest(extractErr.Error())
	}

	idempotencyKey := fmt.Sprintf("dexpay:%s:%s", event.Event, providerRef)

	exists, err := s.webhookRepo.Exists(ctx, idempotencyKey)
	if err != nil {
		return domain.InternalError(err)
	}
	if exists {
		slog.Info("webhook event already processed", "event", event.Event, "ref", providerRef)
		return nil
	}

	var processErr *domain.AppError
	switch event.Event {
	case dexpay.EventCheckoutInitiated:
		processErr = s.handleCheckoutInitiated(ctx, event.Data)
	case dexpay.EventCheckoutCompleted:
		processErr = s.handleCheckoutCompleted(ctx, event.Data)
	case dexpay.EventCheckoutFailed:
		processErr = s.handleCheckoutFailed(ctx, event.Data, "failed")
	case dexpay.EventCheckoutCancelled:
		processErr = s.handleCheckoutFailed(ctx, event.Data, "cancelled")
	case dexpay.EventCheckoutRefunded:
		processErr = s.handleCheckoutRefunded(ctx, event.Data)
	default:
		slog.Warn("unhandled webhook event", "event", event.Event)
	}
	if processErr != nil {
		return processErr
	}

	webhookEvent := &domain.WebhookEvent{
		Provider:       dexpayProviderName,
		EventType:      event.Event,
		ProviderRef:    providerRef,
		IdempotencyKey: idempotencyKey,
		Payload:        rawPayload,
	}
	if err := s.webhookRepo.Create(ctx, webhookEvent); err != nil {
		slog.Error("failed to record webhook event", "error", err, "key", idempotencyKey)
	}

	return nil
}

// List returns payments for the authenticated user.
func (s *PaymentService) List(ctx context.Context, ac *auth.AuthContext, limit, offset int) ([]domain.Payment, *domain.AppError) {
	payments, err := s.repo.ListByOrgAndUser(ctx, ac.OrgID, ac.UserID, limit, offset)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	for i := range payments {
		if appErr := s.refreshPaymentState(ctx, &payments[i], false); appErr != nil {
			return nil, appErr
		}
	}
	return payments, nil
}

// Get returns a single payment, verifying ownership.
func (s *PaymentService) Get(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*domain.Payment, *domain.AppError) {
	payment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if payment == nil || payment.OrgID != ac.OrgID {
		return nil, domain.NotFound("payment")
	}
	if appErr := s.refreshPaymentState(ctx, payment, true); appErr != nil {
		return nil, appErr
	}
	return payment, nil
}

// Resume reuses or recreates a checkout session for an existing payment attempt.
func (s *PaymentService) Resume(ctx context.Context, ac *auth.AuthContext, id uuid.UUID) (*CheckoutResult, *domain.AppError) {
	payment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if payment == nil || payment.OrgID != ac.OrgID || payment.UserID != ac.UserID {
		return nil, domain.NotFound("payment")
	}

	if appErr := s.refreshPaymentState(ctx, payment, true); appErr != nil {
		return nil, appErr
	}

	attempts, err := s.repo.ListBySubscriptionID(ctx, payment.SubscriptionID, 10)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	for i := range attempts {
		candidate := &attempts[i]
		if candidate.OrgID != ac.OrgID || candidate.UserID != ac.UserID {
			continue
		}
		if candidate.ID != payment.ID {
			if appErr := s.refreshPaymentState(ctx, candidate, true); appErr != nil {
				return nil, appErr
			}
		}
		switch candidate.Status {
		case domain.PaymentStatusCompleted, domain.PaymentStatusRefunded:
			return s.buildCheckoutResult(ctx, candidate)
		case domain.PaymentStatusPending:
			if candidate.PaymentURL != nil && !paymentExpired(candidate) {
				return s.buildCheckoutResult(ctx, candidate)
			}
		}
	}

	switch payment.Status {
	case domain.PaymentStatusCompleted, domain.PaymentStatusRefunded:
		return s.buildCheckoutResult(ctx, payment)
	case domain.PaymentStatusPending:
		if payment.PaymentURL != nil && !paymentExpired(payment) {
			return s.buildCheckoutResult(ctx, payment)
		}
	}

	sub, err := s.subRepo.GetByID(ctx, payment.SubscriptionID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if sub == nil || sub.OrgID != ac.OrgID {
		return nil, domain.NotFound("subscription")
	}

	device, err := s.deviceRepo.GetByID(ctx, sub.DeviceID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if device == nil || device.OrgID != ac.OrgID {
		return nil, domain.NotFound("device")
	}

	plan, err := s.planRepo.GetByID(ctx, payment.PlanID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if plan == nil {
		return nil, domain.NotFound("plan")
	}

	attempt, err := s.createPaymentAttempt(ctx, ac, sub, payment.AmountXOF, nil)
	if err != nil {
		return nil, domain.InternalError(err)
	}

	return s.initiateDexpayCheckout(ctx, ac, device, sub, attempt, plan)
}

func (s *PaymentService) initiateDexpayCheckout(ctx context.Context, ac *auth.AuthContext,
	device *domain.Device, sub *domain.Subscription, payment *domain.Payment, plan *domain.Plan,
) (*CheckoutResult, *domain.AppError) {
	user, err := s.userRepo.GetByID(ctx, ac.UserID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if user == nil {
		return nil, domain.NotFound("user")
	}

	req := s.buildCheckoutSessionRequest(user, plan, sub, payment)
	session, err := s.dexpayClient.CreateCheckoutSession(ctx, req)
	if err != nil {
		session, err = s.recoverCheckoutSession(ctx, providerRefValue(payment), err)
		if err != nil {
			s.markCheckoutInitiationFailed(ctx, payment, err)
			return nil, domain.PaymentGatewayError(fmt.Errorf("create checkout session: %w", err))
		}
	}

	if err := s.applyCheckoutSessionToPayment(ctx, payment, session); err != nil {
		return nil, domain.InternalError(err)
	}

	return &CheckoutResult{
		Payment:      payment,
		Device:       device,
		Subscription: sub,
		PaymentURL:   valueOrEmpty(payment.PaymentURL),
	}, nil
}

func (s *PaymentService) buildCheckoutSessionRequest(
	user *domain.User,
	plan *domain.Plan,
	sub *domain.Subscription,
	payment *domain.Payment,
) dexpay.CreateCheckoutSessionRequest {
	return dexpay.CreateCheckoutSessionRequest{
		Reference:  providerRefValue(payment),
		ItemName:   buildCheckoutItemName(plan, sub.BillingCycle),
		Amount:     payment.AmountXOF,
		Currency:   payment.Currency,
		CountryISO: dexpayCountrySN,
		WebhookURL: s.backendPublicURL + "/api/v1/webhooks/dexpay",
		SuccessURL: buildFrontendCallbackURL(s.frontendURL, "/paiement/succes", payment),
		FailureURL: buildFrontendCallbackURL(s.frontendURL, "/paiement/echec", payment),
		Customer:   buildCheckoutCustomer(user),
	}
}

func buildCheckoutCustomer(user *domain.User) *dexpay.CheckoutSessionCustomer {
	customer := &dexpay.CheckoutSessionCustomer{
		Name:  strings.TrimSpace(user.FullName),
		Email: strings.TrimSpace(user.Email),
	}
	if user.Phone != nil {
		customer.Phone = strings.TrimSpace(*user.Phone)
	}
	if customer.Name == "" && customer.Email == "" && customer.Phone == "" {
		return nil
	}
	return customer
}

func buildCheckoutItemName(plan *domain.Plan, billingCycle string) string {
	name := strings.TrimSpace(plan.NameFR)
	if name == "" {
		name = strings.TrimSpace(plan.NameEN)
	}
	if name == "" {
		name = "SafePhone"
	}
	if billingCycle == "annual" {
		return fmt.Sprintf("SafePhone %s (annual)", name)
	}
	return fmt.Sprintf("SafePhone %s (monthly)", name)
}

func buildFrontendCallbackURL(baseURL, path string, payment *domain.Payment) string {
	return fmt.Sprintf(
		"%s%s?payment_id=%s&reference=%s",
		baseURL,
		path,
		payment.ID.String(),
		providerRefValue(payment),
	)
}

func (s *PaymentService) recoverCheckoutSession(ctx context.Context, reference string, createErr error) (*dexpay.CheckoutSession, error) {
	var apiErr *dexpay.APIError
	shouldRecover := false
	if errors.As(createErr, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusConflict:
			shouldRecover = true
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusBadRequest:
			return nil, createErr
		}
	} else {
		shouldRecover = true
	}

	if !shouldRecover {
		return nil, createErr
	}

	session, err := s.dexpayClient.GetCheckoutSession(ctx, reference)
	if err != nil {
		var lookupErr *dexpay.APIError
		if errors.As(err, &lookupErr) && lookupErr.StatusCode == http.StatusNotFound {
			return nil, createErr
		}
		return nil, fmt.Errorf("recovery lookup for checkout session %s failed after create error: %w", reference, err)
	}

	slog.Warn("recovered DEXPAY checkout session after create failure",
		"reference", reference,
		"environment", s.dexpayClient.Environment(),
	)
	return session, nil
}

func (s *PaymentService) applyCheckoutSessionToPayment(ctx context.Context, payment *domain.Payment, session *dexpay.CheckoutSession) error {
	if session == nil {
		return fmt.Errorf("missing DEXPAY checkout session response")
	}

	paymentURL := session.PaymentURL
	if s.dexpayClient != nil && s.dexpayClient.IsSandbox() && session.SandboxPaymentURL != "" {
		paymentURL = session.SandboxPaymentURL
	}
	if paymentURL != "" {
		payment.PaymentURL = &paymentURL
	}
	if session.Reference != "" {
		payment.ProviderRef = &session.Reference
	}
	if expiresAt := parseDexpayTimestamp(session.ExpiresAt); expiresAt != nil {
		payment.ExpiresAt = expiresAt
	}
	applyCheckoutStatusToPayment(payment, session.Status)

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal checkout session payload: %w", err)
	}
	applyConfirmedPaymentMethod(payment, payload)
	payment.ProviderPayload = payload

	return s.repo.Update(ctx, payment)
}

func (s *PaymentService) buildCheckoutResult(ctx context.Context, payment *domain.Payment) (*CheckoutResult, *domain.AppError) {
	result := &CheckoutResult{Payment: payment}

	sub, err := s.subRepo.GetByID(ctx, payment.SubscriptionID)
	if err != nil {
		return nil, domain.InternalError(err)
	}
	if sub != nil {
		result.Subscription = sub
		device, err := s.deviceRepo.GetByID(ctx, sub.DeviceID)
		if err != nil {
			return nil, domain.InternalError(err)
		}
		result.Device = device
	}

	if appErr := s.refreshPaymentState(ctx, payment, false); appErr != nil {
		return nil, appErr
	}

	result.PaymentURL = valueOrEmpty(payment.PaymentURL)
	return result, nil
}

func (s *PaymentService) handleCheckoutInitiated(ctx context.Context, data json.RawMessage) *domain.AppError {
	payment, _, _, d, appErr := s.loadPaymentCoverageByReference(ctx, data)
	if appErr != nil || payment == nil {
		return appErr
	}

	if paymentURL := webhookPaymentURL(d); paymentURL != "" && payment.PaymentURL == nil {
		payment.PaymentURL = &paymentURL
	}
	applyCheckoutStatusToPayment(payment, d.Status)
	applyConfirmedPaymentMethod(payment, data)
	payment.ProviderPayload = append([]byte(nil), data...)
	if err := s.repo.Update(ctx, payment); err != nil {
		return domain.InternalError(err)
	}

	slog.Info("checkout initiated via webhook", "payment_id", payment.ID, "reference", d.Reference)
	return nil
}

func (s *PaymentService) handleCheckoutCompleted(ctx context.Context, data json.RawMessage) *domain.AppError {
	payment, sub, device, d, appErr := s.loadPaymentCoverageByReference(ctx, data)
	if appErr != nil || payment == nil {
		return appErr
	}

	if payment.Status != domain.PaymentStatusRefunded {
		now := time.Now()
		payment.Status = domain.PaymentStatusCompleted
		payment.PaidAt = &now
		payment.FailedAt = nil
		if paymentURL := webhookPaymentURL(d); paymentURL != "" && payment.PaymentURL == nil {
			payment.PaymentURL = &paymentURL
		}
		applyCheckoutStatusToPayment(payment, d.Status)
		applyConfirmedPaymentMethod(payment, data)
		payment.ProviderPayload = append([]byte(nil), data...)
		if err := s.repo.Update(ctx, payment); err != nil {
			return domain.InternalError(err)
		}
	}

	if appErr := s.activateCoverage(ctx, sub, device); appErr != nil {
		return appErr
	}
	s.syncPartnerClientStatus(ctx, payment.UserID, "active", &payment.PlanID)

	slog.Info("checkout completed via webhook", "payment_id", payment.ID, "reference", d.Reference)
	return nil
}

func (s *PaymentService) handleCheckoutFailed(ctx context.Context, data json.RawMessage, reason string) *domain.AppError {
	payment, _, _, d, appErr := s.loadPaymentCoverageByReference(ctx, data)
	if appErr != nil || payment == nil {
		return appErr
	}

	if payment.Status == domain.PaymentStatusPending {
		if paymentURL := webhookPaymentURL(d); paymentURL != "" && payment.PaymentURL == nil {
			payment.PaymentURL = &paymentURL
		}
		switch strings.ToLower(strings.TrimSpace(reason)) {
		case "cancelled", "canceled":
			payment.Status = domain.PaymentStatusCancelled
		default:
			payment.Status = domain.PaymentStatusFailed
		}
		now := time.Now()
		payment.FailedAt = &now
		applyCheckoutStatusToPayment(payment, d.Status)
		applyConfirmedPaymentMethod(payment, data)
		payment.ProviderPayload = append([]byte(nil), data...)
		if err := s.repo.Update(ctx, payment); err != nil {
			return domain.InternalError(err)
		}
	}

	slog.Info("checkout not completed via webhook", "payment_id", payment.ID, "reference", d.Reference, "status", reason)
	return nil
}

func (s *PaymentService) handleCheckoutRefunded(ctx context.Context, data json.RawMessage) *domain.AppError {
	payment, _, _, d, appErr := s.loadPaymentCoverageByReference(ctx, data)
	if appErr != nil || payment == nil {
		return appErr
	}

	if payment.Status != domain.PaymentStatusRefunded {
		payment.Status = domain.PaymentStatusRefunded
		applyCheckoutStatusToPayment(payment, d.Status)
		applyConfirmedPaymentMethod(payment, data)
		payment.ProviderPayload = append([]byte(nil), data...)
		if err := s.repo.Update(ctx, payment); err != nil {
			return domain.InternalError(err)
		}
	}

	slog.Info("checkout refunded via webhook", "payment_id", payment.ID, "reference", d.Reference)
	return nil
}

func (s *PaymentService) loadPaymentCoverageByReference(ctx context.Context, data json.RawMessage) (
	*domain.Payment,
	*domain.Subscription,
	*domain.Device,
	*dexpay.CheckoutWebhookData,
	*domain.AppError,
) {
	d, err := parseCheckoutWebhookData(data)
	if err != nil {
		return nil, nil, nil, nil, domain.BadRequest(err.Error())
	}

	payment, err := s.repo.GetByProviderRef(ctx, d.Reference)
	if err != nil {
		return nil, nil, nil, nil, domain.InternalError(err)
	}
	if payment == nil {
		slog.Warn("payment not found for checkout webhook", "reference", d.Reference)
		return nil, nil, nil, d, nil
	}

	sub, err := s.subRepo.GetByID(ctx, payment.SubscriptionID)
	if err != nil {
		return nil, nil, nil, nil, domain.InternalError(err)
	}
	if sub == nil {
		slog.Warn("subscription not found for checkout webhook", "reference", d.Reference, "payment_id", payment.ID)
		return payment, nil, nil, d, nil
	}

	device, err := s.deviceRepo.GetByID(ctx, sub.DeviceID)
	if err != nil {
		return nil, nil, nil, nil, domain.InternalError(err)
	}

	return payment, sub, device, d, nil
}

func parseCheckoutWebhookData(data json.RawMessage) (*dexpay.CheckoutWebhookData, error) {
	var parsed dexpay.CheckoutWebhookData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("invalid checkout webhook payload")
	}
	if strings.TrimSpace(parsed.Reference) == "" {
		return nil, fmt.Errorf("checkout webhook payload is missing reference")
	}
	return &parsed, nil
}

func (s *PaymentService) activateCoverage(ctx context.Context, sub *domain.Subscription, device *domain.Device) *domain.AppError {
	if sub != nil {
		periodStart, periodEnd := subscriptionCoverageWindow(sub.BillingCycle, time.Now())
		sub.Status = domain.SubscriptionStatusActive
		sub.CurrentPeriodStart = &periodStart
		sub.CurrentPeriodEnd = &periodEnd
		if err := s.subRepo.Update(ctx, sub); err != nil {
			return domain.InternalError(err)
		}
	}

	if device != nil {
		nextStatus := domain.DeviceStatusPending
		if sub != nil && sub.Status == domain.SubscriptionStatusActive && strings.TrimSpace(device.IMEI) != "" {
			nextStatus = domain.DeviceStatusActive
		}
		if device.Status != nextStatus {
			device.Status = nextStatus
			if err := s.deviceRepo.Update(ctx, device); err != nil {
				return domain.InternalError(err)
			}
		}
	}

	return nil
}

func (s *PaymentService) createPaymentAttempt(ctx context.Context, ac *auth.AuthContext, sub *domain.Subscription, amount int, idempotencyKey *string) (*domain.Payment, error) {
	if sub == nil {
		return nil, fmt.Errorf("missing subscription")
	}

	paymentID := uuid.New()
	providerRef := buildCheckoutReference(paymentID)
	payment := &domain.Payment{
		ID:             paymentID,
		OrgID:          ac.OrgID,
		UserID:         ac.UserID,
		PlanID:         sub.PlanID,
		SubscriptionID: sub.ID,
		AmountXOF:      amount,
		Currency:       dexpayCurrencyXOF,
		Provider:       dexpayProviderName,
		Status:         domain.PaymentStatusPending,
		ProviderRef:    &providerRef,
		IdempotencyKey: idempotencyKey,
	}

	if err := s.repo.Create(ctx, payment); err != nil {
		return nil, err
	}

	return payment, nil
}

func (s *PaymentService) refreshPaymentState(ctx context.Context, payment *domain.Payment, syncWithProvider bool) *domain.AppError {
	if payment == nil {
		return nil
	}

	if payment.Status == domain.PaymentStatusPending && paymentExpired(payment) {
		now := time.Now()
		payment.Status = domain.PaymentStatusExpired
		payment.FailedAt = &now
		if err := s.repo.Update(ctx, payment); err != nil {
			return domain.InternalError(err)
		}
		return nil
	}

	shouldFetchProvider := payment.Status == domain.PaymentStatusPending &&
		s.dexpayClient != nil &&
		payment.ProviderRef != nil &&
		(syncWithProvider || payment.PaymentURL == nil)

	if !shouldFetchProvider {
		return nil
	}

	session, err := s.dexpayClient.GetCheckoutSession(ctx, *payment.ProviderRef)
	if err != nil {
		var apiErr *dexpay.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			slog.Warn("failed to refresh payment checkout session", "payment_id", payment.ID, "error", err)
		}
		return nil
	}

	previousStatus := payment.Status
	if err := s.applyCheckoutSessionToPayment(ctx, payment, session); err != nil {
		return domain.InternalError(err)
	}

	if payment.Status == domain.PaymentStatusCompleted && previousStatus != domain.PaymentStatusCompleted {
		sub, device, appErr := s.loadCoverageEntitiesForPayment(ctx, payment)
		if appErr != nil {
			return appErr
		}
		if appErr := s.activateCoverage(ctx, sub, device); appErr != nil {
			return appErr
		}
		s.syncPartnerClientStatus(ctx, payment.UserID, "active", &payment.PlanID)
	}

	return nil
}

func (s *PaymentService) loadCoverageEntitiesForPayment(ctx context.Context, payment *domain.Payment) (*domain.Subscription, *domain.Device, *domain.AppError) {
	if payment == nil {
		return nil, nil, nil
	}

	sub, err := s.subRepo.GetByID(ctx, payment.SubscriptionID)
	if err != nil {
		return nil, nil, domain.InternalError(err)
	}
	if sub == nil {
		return nil, nil, nil
	}

	device, err := s.deviceRepo.GetByID(ctx, sub.DeviceID)
	if err != nil {
		return nil, nil, domain.InternalError(err)
	}

	return sub, device, nil
}

func subscriptionCoverageWindow(billingCycle string, now time.Time) (time.Time, time.Time) {
	periodStart := now
	if billingCycle == "annual" {
		return periodStart, now.AddDate(1, 0, 0)
	}
	return periodStart, now.AddDate(0, 1, 0)
}

func paymentExpired(payment *domain.Payment) bool {
	return payment != nil && payment.ExpiresAt != nil && time.Now().After(*payment.ExpiresAt)
}

func applyConfirmedPaymentMethod(payment *domain.Payment, payload []byte) {
	if payment == nil || len(payload) == 0 {
		return
	}

	method := extractConfirmedPaymentMethod(payload)
	if method == "" {
		return
	}

	payment.PaymentMethod = &method
}

func extractConfirmedPaymentMethod(payload []byte) string {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ""
	}

	for _, key := range []string{
		"payment_method",
		"paymentMethod",
		"payment_channel",
		"paymentChannel",
		"payment_network",
		"paymentNetwork",
		"payment_type",
		"paymentType",
		"channel",
		"method",
	} {
		if value, ok := raw[key]; ok {
			if method := strings.TrimSpace(fmt.Sprint(value)); method != "" && method != "<nil>" {
				return method
			}
		}
	}

	return ""
}

func parseDexpayTimestamp(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed
		}
	}
	return nil
}

func applyCheckoutStatusToPayment(payment *domain.Payment, rawStatus string) {
	if payment == nil {
		return
	}

	now := time.Now()
	switch strings.ToLower(strings.TrimSpace(rawStatus)) {
	case "", "pending", "created", "initiated", "processing":
		return
	case "completed", "paid", "successful", "success":
		if payment.Status != domain.PaymentStatusRefunded {
			payment.Status = domain.PaymentStatusCompleted
			payment.PaidAt = &now
			payment.FailedAt = nil
		}
	case "failed":
		if payment.Status == domain.PaymentStatusPending {
			payment.Status = domain.PaymentStatusFailed
			payment.FailedAt = &now
		}
	case "cancelled", "canceled":
		if payment.Status == domain.PaymentStatusPending {
			payment.Status = domain.PaymentStatusCancelled
			payment.FailedAt = &now
		}
	case "expired":
		if payment.Status == domain.PaymentStatusPending {
			payment.Status = domain.PaymentStatusExpired
			payment.FailedAt = &now
		}
	case "refunded":
		payment.Status = domain.PaymentStatusRefunded
	}
}

// extractProviderRef extracts a unique reference from the event data for idempotency.
func extractProviderRef(event dexpay.WebhookEvent) (string, error) {
	var raw map[string]any
	if err := json.Unmarshal(event.Data, &raw); err != nil {
		return "", fmt.Errorf("invalid event data")
	}

	for _, key := range []string{"reference", "checkout_session_id", "payment_id", "transaction_id"} {
		if v, ok := raw[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, nil
			}
		}
	}

	return "", fmt.Errorf("no provider reference found in event data")
}

func buildCheckoutReference(paymentID uuid.UUID) string {
	return "SPAY_" + paymentID.String()
}

func providerRefValue(payment *domain.Payment) string {
	if payment == nil || payment.ProviderRef == nil {
		return ""
	}
	return *payment.ProviderRef
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func webhookPaymentURL(data *dexpay.CheckoutWebhookData) string {
	if data == nil {
		return ""
	}
	if data.SandboxPaymentURL != "" {
		return data.SandboxPaymentURL
	}
	return data.PaymentURL
}

func (s *PaymentService) markCheckoutInitiationFailed(ctx context.Context, payment *domain.Payment, cause error) {
	if payment == nil {
		return
	}

	now := time.Now()
	payment.Status = domain.PaymentStatusFailed
	payment.FailedAt = &now

	payload, err := json.Marshal(map[string]string{
		"stage": "create_checkout_session",
		"error": cause.Error(),
	})
	if err == nil {
		payment.ProviderPayload = payload
	}

	if updateErr := s.repo.Update(ctx, payment); updateErr != nil {
		slog.Warn("failed to mark checkout initiation failure", "payment_id", payment.ID, "error", updateErr)
	}
}

func (s *PaymentService) syncPartnerClientStatus(ctx context.Context, userID uuid.UUID, status string, planID *uuid.UUID) {
	if s.partnerRepo == nil {
		return
	}
	if err := s.partnerRepo.UpdateLatestClientStatusByLinkedUser(ctx, userID, status, planID); err != nil {
		slog.Warn("failed to sync partner client status", "user_id", userID, "status", status, "error", err)
	}
}
