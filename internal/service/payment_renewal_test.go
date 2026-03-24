package service

import (
	"context"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/auth"
	"github.com/cherif-safephone/safephone-backend/internal/dexpay"
	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

type renewalRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn renewalRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRenewSubscriptionCreatesPendingCheckoutForExpiredSubscription(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	deviceID := uuid.New()
	expiredSubscriptionID := uuid.New()
	oldPlanID := uuid.New()
	newPlanID := uuid.New()

	subRepo := &stubRenewalSubscriptionRepository{
		subs: map[uuid.UUID]*domain.Subscription{
			expiredSubscriptionID: {
				ID:                 expiredSubscriptionID,
				OrgID:              orgID,
				UserID:             userID,
				DeviceID:           deviceID,
				PlanID:             oldPlanID,
				Status:             domain.SubscriptionStatusExpired,
				BillingCycle:       "monthly",
				CurrentPeriodStart: timePointer(time.Now().AddDate(0, -1, 0)),
				CurrentPeriodEnd:   timePointer(time.Now().Add(-time.Hour)),
				CreatedAt:          time.Now().AddDate(0, -1, 0),
				UpdatedAt:          time.Now().Add(-time.Hour),
			},
		},
	}
	deviceRepo := &stubRenewalDeviceRepository{
		devices: map[uuid.UUID]*domain.Device{
			deviceID: {
				ID:         deviceID,
				OrgID:      orgID,
				UserID:     userID,
				DeviceType: domain.DeviceTypeSmartphone,
				Brand:      "Apple",
				Model:      "iPhone 14",
				Status:     domain.DeviceStatusExpired,
			},
		},
	}
	planRepo := &stubRenewalPlanRepository{
		plans: map[uuid.UUID]*domain.Plan{
			newPlanID: {
				ID:           newPlanID,
				Slug:         "ecran-plus",
				NameFR:       "Ecran+",
				NameEN:       "Screen+",
				PriceMonthly: 3500,
				PriceAnnual:  36000,
			},
		},
	}
	paymentRepo := &stubRenewalPaymentRepository{payments: map[uuid.UUID]*domain.Payment{}}
	userRepo := &stubRenewalUserRepository{
		users: map[uuid.UUID]*domain.User{
			userID: {
				ID:       userID,
				OrgID:    orgID,
				FullName: "Renew Test",
				Email:    "renew@example.com",
			},
		},
	}

	client := dexpay.NewClient("https://api.safephone.test", "pk_test_public", time.Second)
	client.SetHTTPClient(&http.Client{
		Timeout: time.Second,
		Transport: renewalRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost || r.URL.Path != "/checkout-sessions" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"status":201,"message":"Checkout session created successfully","data":{"reference":"SPAY_RENEW_123","amount":36000,"currency":"XOF","status":"pending","payment_url":"https://pay.example/checkout/SPAY_RENEW_123","isSandbox":false}}`,
				)),
			}, nil
		}),
	})

	svc := &PaymentService{
		repo:             paymentRepo,
		subRepo:          subRepo,
		planRepo:         planRepo,
		userRepo:         userRepo,
		deviceRepo:       deviceRepo,
		dexpayClient:     client,
		frontendURL:      "https://app.safephone.test",
		backendPublicURL: "https://api.safephone.test",
		devMode:          false,
	}

	result, appErr := svc.RenewSubscription(
		context.Background(),
		&auth.AuthContext{OrgID: orgID, UserID: userID},
		expiredSubscriptionID,
		newPlanID,
		"annual",
		nil,
	)
	if appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}
	if result == nil || result.Subscription == nil || result.Payment == nil {
		t.Fatalf("expected checkout result with payment and subscription, got %#v", result)
	}
	if result.Subscription.ID == expiredSubscriptionID {
		t.Fatalf("expected a new subscription, got original %#v", result.Subscription)
	}
	if result.Subscription.Status != domain.SubscriptionStatusPending {
		t.Fatalf("expected pending renewal subscription, got %q", result.Subscription.Status)
	}
	if result.Subscription.DeviceID != deviceID {
		t.Fatalf("expected device %s, got %s", deviceID, result.Subscription.DeviceID)
	}
	if result.Subscription.PlanID != newPlanID {
		t.Fatalf("expected plan %s, got %s", newPlanID, result.Subscription.PlanID)
	}
	if result.Subscription.BillingCycle != "annual" {
		t.Fatalf("expected annual billing cycle, got %q", result.Subscription.BillingCycle)
	}
	if result.Payment.Status != domain.PaymentStatusPending {
		t.Fatalf("expected pending payment, got %q", result.Payment.Status)
	}
	if result.Payment.SubscriptionID != result.Subscription.ID {
		t.Fatalf("expected payment linked to new subscription, got %s", result.Payment.SubscriptionID)
	}
	if result.PaymentURL != "https://pay.example/checkout/SPAY_RENEW_123" {
		t.Fatalf("expected checkout URL, got %q", result.PaymentURL)
	}

	sourceAfter, err := subRepo.GetByID(context.Background(), expiredSubscriptionID)
	if err != nil {
		t.Fatalf("expected source subscription lookup to succeed, got %v", err)
	}
	if sourceAfter == nil || sourceAfter.Status != domain.SubscriptionStatusExpired {
		t.Fatalf("expected original subscription to remain expired, got %#v", sourceAfter)
	}
}

func TestRenewSubscriptionActivatesCoverageInDevMode(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	deviceID := uuid.New()
	expiredSubscriptionID := uuid.New()
	planID := uuid.New()

	subRepo := &stubRenewalSubscriptionRepository{
		subs: map[uuid.UUID]*domain.Subscription{
			expiredSubscriptionID: {
				ID:           expiredSubscriptionID,
				OrgID:        orgID,
				UserID:       userID,
				DeviceID:     deviceID,
				PlanID:       planID,
				Status:       domain.SubscriptionStatusExpired,
				BillingCycle: "monthly",
				CreatedAt:    time.Now().AddDate(0, -1, 0),
			},
		},
	}
	deviceRepo := &stubRenewalDeviceRepository{
		devices: map[uuid.UUID]*domain.Device{
			deviceID: {
				ID:         deviceID,
				OrgID:      orgID,
				UserID:     userID,
				DeviceType: domain.DeviceTypeSmartphone,
				Brand:      "Samsung",
				Model:      "S24",
				IMEI:       "356789012345678",
				Status:     domain.DeviceStatusExpired,
			},
		},
	}
	planRepo := &stubRenewalPlanRepository{
		plans: map[uuid.UUID]*domain.Plan{
			planID: {
				ID:           planID,
				Slug:         "essentiel",
				NameFR:       "Essentiel",
				NameEN:       "Essential",
				PriceMonthly: 1500,
				PriceAnnual:  15000,
			},
		},
	}

	svc := &PaymentService{
		repo:        &stubRenewalPaymentRepository{payments: map[uuid.UUID]*domain.Payment{}},
		subRepo:     subRepo,
		planRepo:    planRepo,
		deviceRepo:  deviceRepo,
		partnerRepo: nil,
		devMode:     true,
	}

	result, appErr := svc.RenewSubscription(
		context.Background(),
		&auth.AuthContext{OrgID: orgID, UserID: userID},
		expiredSubscriptionID,
		planID,
		"monthly",
		nil,
	)
	if appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}
	if result.Subscription == nil || result.Subscription.Status != domain.SubscriptionStatusActive {
		t.Fatalf("expected active renewed subscription in dev mode, got %#v", result.Subscription)
	}
	if result.Payment == nil || result.Payment.Status != domain.PaymentStatusCompleted {
		t.Fatalf("expected completed payment in dev mode, got %#v", result.Payment)
	}
	deviceAfter, err := deviceRepo.GetByID(context.Background(), deviceID)
	if err != nil {
		t.Fatalf("expected device lookup to succeed, got %v", err)
	}
	if deviceAfter == nil || deviceAfter.Status != domain.DeviceStatusActive {
		t.Fatalf("expected device to be active after successful renewal, got %#v", deviceAfter)
	}
}

func TestRenewSubscriptionRejectsWhenSourceNotExpired(t *testing.T) {
	t.Parallel()

	for _, status := range []domain.SubscriptionStatus{
		domain.SubscriptionStatusActive,
		domain.SubscriptionStatusPending,
		domain.SubscriptionStatusCancelled,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			orgID := uuid.New()
			userID := uuid.New()
			deviceID := uuid.New()
			subscriptionID := uuid.New()
			planID := uuid.New()

			svc := &PaymentService{
				repo: &stubRenewalPaymentRepository{payments: map[uuid.UUID]*domain.Payment{}},
				subRepo: &stubRenewalSubscriptionRepository{
					subs: map[uuid.UUID]*domain.Subscription{
						subscriptionID: {
							ID:           subscriptionID,
							OrgID:        orgID,
							UserID:       userID,
							DeviceID:     deviceID,
							PlanID:       planID,
							Status:       status,
							BillingCycle: "monthly",
						},
					},
				},
				planRepo: &stubRenewalPlanRepository{
					plans: map[uuid.UUID]*domain.Plan{
						planID: {ID: planID, Slug: "essentiel", PriceMonthly: 1000, PriceAnnual: 10000},
					},
				},
				deviceRepo: &stubRenewalDeviceRepository{
					devices: map[uuid.UUID]*domain.Device{
						deviceID: {ID: deviceID, OrgID: orgID, UserID: userID},
					},
				},
				devMode: true,
			}

			_, appErr := svc.RenewSubscription(
				context.Background(),
				&auth.AuthContext{OrgID: orgID, UserID: userID},
				subscriptionID,
				planID,
				"monthly",
				nil,
			)
			if appErr == nil {
				t.Fatalf("expected app error for status %q", status)
			}
			if appErr.Code != domain.CodeBadRequest {
				t.Fatalf("expected bad request for status %q, got %#v", status, appErr)
			}
		})
	}
}

func TestRenewSubscriptionRejectsUnavailablePlan(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	deviceID := uuid.New()
	subscriptionID := uuid.New()

	svc := &PaymentService{
		repo: &stubRenewalPaymentRepository{payments: map[uuid.UUID]*domain.Payment{}},
		subRepo: &stubRenewalSubscriptionRepository{
			subs: map[uuid.UUID]*domain.Subscription{
				subscriptionID: {
					ID:       subscriptionID,
					OrgID:    orgID,
					UserID:   userID,
					DeviceID: deviceID,
					Status:   domain.SubscriptionStatusExpired,
				},
			},
		},
		planRepo: &stubRenewalPlanRepository{plans: map[uuid.UUID]*domain.Plan{}},
		deviceRepo: &stubRenewalDeviceRepository{
			devices: map[uuid.UUID]*domain.Device{
				deviceID: {ID: deviceID, OrgID: orgID, UserID: userID},
			},
		},
		devMode: true,
	}

	_, appErr := svc.RenewSubscription(
		context.Background(),
		&auth.AuthContext{OrgID: orgID, UserID: userID},
		subscriptionID,
		uuid.New(),
		"monthly",
		nil,
	)
	if appErr == nil {
		t.Fatal("expected app error for unavailable plan")
	}
	if appErr.Code != domain.CodeNotFound {
		t.Fatalf("expected not found error, got %#v", appErr)
	}
}

func TestRenewSubscriptionRejectsWhenDeviceAlreadyHasActiveOrPendingCoverage(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	deviceID := uuid.New()
	expiredSubscriptionID := uuid.New()
	otherSubscriptionID := uuid.New()
	planID := uuid.New()

	for _, status := range []domain.SubscriptionStatus{
		domain.SubscriptionStatusActive,
		domain.SubscriptionStatusPending,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			subRepo := &stubRenewalSubscriptionRepository{
				subs: map[uuid.UUID]*domain.Subscription{
					expiredSubscriptionID: {
						ID:        expiredSubscriptionID,
						OrgID:     orgID,
						UserID:    userID,
						DeviceID:  deviceID,
						PlanID:    planID,
						Status:    domain.SubscriptionStatusExpired,
						CreatedAt: time.Now().Add(-2 * time.Hour),
					},
					otherSubscriptionID: {
						ID:        otherSubscriptionID,
						OrgID:     orgID,
						UserID:    userID,
						DeviceID:  deviceID,
						PlanID:    planID,
						Status:    status,
						CreatedAt: time.Now().Add(-time.Hour),
					},
				},
			}

			svc := &PaymentService{
				repo:    &stubRenewalPaymentRepository{payments: map[uuid.UUID]*domain.Payment{}},
				subRepo: subRepo,
				planRepo: &stubRenewalPlanRepository{
					plans: map[uuid.UUID]*domain.Plan{
						planID: {ID: planID, Slug: "essentiel", PriceMonthly: 1000, PriceAnnual: 10000},
					},
				},
				deviceRepo: &stubRenewalDeviceRepository{
					devices: map[uuid.UUID]*domain.Device{
						deviceID: {ID: deviceID, OrgID: orgID, UserID: userID},
					},
				},
				devMode: true,
			}

			_, appErr := svc.RenewSubscription(
				context.Background(),
				&auth.AuthContext{OrgID: orgID, UserID: userID},
				expiredSubscriptionID,
				planID,
				"monthly",
				nil,
			)
			if appErr == nil {
				t.Fatalf("expected conflict for existing %q subscription", status)
			}
			if appErr.Code != domain.CodeConflict {
				t.Fatalf("expected conflict error, got %#v", appErr)
			}
		})
	}
}

type stubRenewalPlanRepository struct {
	plans map[uuid.UUID]*domain.Plan
}

func (s *stubRenewalPlanRepository) List(_ context.Context) ([]domain.Plan, error) { return nil, nil }
func (s *stubRenewalPlanRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.Plan, error) {
	if plan, ok := s.plans[id]; ok {
		clone := *plan
		return &clone, nil
	}
	return nil, nil
}
func (s *stubRenewalPlanRepository) GetBySlug(_ context.Context, _ string) (*domain.Plan, error) {
	return nil, nil
}

type stubRenewalUserRepository struct {
	users map[uuid.UUID]*domain.User
}

func (s *stubRenewalUserRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if user, ok := s.users[id]; ok {
		clone := *user
		return &clone, nil
	}
	return nil, nil
}
func (s *stubRenewalUserRepository) Update(_ context.Context, _ *domain.User) error { return nil }
func (s *stubRenewalUserRepository) UpdateRole(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (s *stubRenewalUserRepository) GetEmployeeProfile(_ context.Context, _, _ uuid.UUID) (*domain.EmployeeProfile, error) {
	return nil, nil
}

type stubRenewalDeviceRepository struct {
	devices map[uuid.UUID]*domain.Device
}

func (s *stubRenewalDeviceRepository) Create(_ context.Context, device *domain.Device) error {
	clone := *device
	if clone.ID == uuid.Nil {
		clone.ID = uuid.New()
	}
	s.devices[clone.ID] = &clone
	device.ID = clone.ID
	return nil
}
func (s *stubRenewalDeviceRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.Device, error) {
	if device, ok := s.devices[id]; ok {
		clone := *device
		return &clone, nil
	}
	return nil, nil
}
func (s *stubRenewalDeviceRepository) GetByIMEI(_ context.Context, _ string) (*domain.Device, error) {
	return nil, nil
}
func (s *stubRenewalDeviceRepository) ListByOrgAndUser(_ context.Context, orgID, userID uuid.UUID, _, _ int) ([]domain.Device, error) {
	var devices []domain.Device
	for _, device := range s.devices {
		if device.OrgID == orgID && device.UserID == userID {
			devices = append(devices, *device)
		}
	}
	return devices, nil
}
func (s *stubRenewalDeviceRepository) Update(_ context.Context, device *domain.Device) error {
	clone := *device
	s.devices[clone.ID] = &clone
	return nil
}
func (s *stubRenewalDeviceRepository) SoftDelete(_ context.Context, id uuid.UUID) error {
	delete(s.devices, id)
	return nil
}

type stubRenewalSubscriptionRepository struct {
	subs map[uuid.UUID]*domain.Subscription
}

func (s *stubRenewalSubscriptionRepository) Create(_ context.Context, sub *domain.Subscription) error {
	clone := *sub
	if clone.ID == uuid.Nil {
		clone.ID = uuid.New()
	}
	now := time.Now()
	if clone.CreatedAt.IsZero() {
		clone.CreatedAt = now
	}
	clone.UpdatedAt = now
	s.subs[clone.ID] = &clone
	sub.ID = clone.ID
	sub.CreatedAt = clone.CreatedAt
	sub.UpdatedAt = clone.UpdatedAt
	return nil
}
func (s *stubRenewalSubscriptionRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.Subscription, error) {
	if sub, ok := s.subs[id]; ok {
		clone := *sub
		return &clone, nil
	}
	return nil, nil
}
func (s *stubRenewalSubscriptionRepository) GetByDeviceID(_ context.Context, deviceID uuid.UUID) (*domain.Subscription, error) {
	subs, _ := s.ListByDeviceID(context.Background(), deviceID, 1)
	if len(subs) == 0 {
		return nil, nil
	}
	sub := subs[0]
	return &sub, nil
}
func (s *stubRenewalSubscriptionRepository) ListByDeviceID(_ context.Context, deviceID uuid.UUID, limit int) ([]domain.Subscription, error) {
	var subs []domain.Subscription
	for _, sub := range s.subs {
		if sub.DeviceID == deviceID {
			subs = append(subs, *sub)
		}
	}
	sort.Slice(subs, func(i, j int) bool {
		return subs[i].CreatedAt.After(subs[j].CreatedAt)
	})
	if limit > 0 && len(subs) > limit {
		subs = subs[:limit]
	}
	return subs, nil
}
func (s *stubRenewalSubscriptionRepository) ListByOrgAndUser(_ context.Context, orgID, userID uuid.UUID, _, _ int) ([]domain.Subscription, error) {
	var subs []domain.Subscription
	for _, sub := range s.subs {
		if sub.OrgID == orgID && sub.UserID == userID {
			subs = append(subs, *sub)
		}
	}
	sort.Slice(subs, func(i, j int) bool {
		return subs[i].CreatedAt.After(subs[j].CreatedAt)
	})
	return subs, nil
}
func (s *stubRenewalSubscriptionRepository) Update(_ context.Context, sub *domain.Subscription) error {
	clone := *sub
	clone.UpdatedAt = time.Now()
	s.subs[clone.ID] = &clone
	sub.UpdatedAt = clone.UpdatedAt
	return nil
}

type stubRenewalPaymentRepository struct {
	payments map[uuid.UUID]*domain.Payment
}

func (s *stubRenewalPaymentRepository) Create(_ context.Context, payment *domain.Payment) error {
	clone := *payment
	now := time.Now()
	if clone.CreatedAt.IsZero() {
		clone.CreatedAt = now
	}
	clone.UpdatedAt = now
	s.payments[clone.ID] = &clone
	payment.CreatedAt = clone.CreatedAt
	payment.UpdatedAt = clone.UpdatedAt
	return nil
}
func (s *stubRenewalPaymentRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.Payment, error) {
	if payment, ok := s.payments[id]; ok {
		clone := *payment
		return &clone, nil
	}
	return nil, nil
}
func (s *stubRenewalPaymentRepository) GetByIdempotencyKey(_ context.Context, key string) (*domain.Payment, error) {
	for _, payment := range s.payments {
		if payment.IdempotencyKey != nil && *payment.IdempotencyKey == key {
			clone := *payment
			return &clone, nil
		}
	}
	return nil, nil
}
func (s *stubRenewalPaymentRepository) GetByProviderRef(_ context.Context, providerRef string) (*domain.Payment, error) {
	for _, payment := range s.payments {
		if payment.ProviderRef != nil && *payment.ProviderRef == providerRef {
			clone := *payment
			return &clone, nil
		}
	}
	return nil, nil
}
func (s *stubRenewalPaymentRepository) GetFirstSuccessfulByUser(_ context.Context, _, _ uuid.UUID) (*domain.Payment, error) {
	return nil, nil
}
func (s *stubRenewalPaymentRepository) ListBySubscriptionID(_ context.Context, subscriptionID uuid.UUID, _ int) ([]domain.Payment, error) {
	var payments []domain.Payment
	for _, payment := range s.payments {
		if payment.SubscriptionID == subscriptionID {
			payments = append(payments, *payment)
		}
	}
	sort.Slice(payments, func(i, j int) bool {
		return payments[i].CreatedAt.After(payments[j].CreatedAt)
	})
	return payments, nil
}
func (s *stubRenewalPaymentRepository) ListByOrgAndUser(_ context.Context, orgID, userID uuid.UUID, _, _ int) ([]domain.Payment, error) {
	var payments []domain.Payment
	for _, payment := range s.payments {
		if payment.OrgID == orgID && payment.UserID == userID {
			payments = append(payments, *payment)
		}
	}
	sort.Slice(payments, func(i, j int) bool {
		return payments[i].CreatedAt.After(payments[j].CreatedAt)
	})
	return payments, nil
}
func (s *stubRenewalPaymentRepository) Update(_ context.Context, payment *domain.Payment) error {
	clone := *payment
	clone.UpdatedAt = time.Now()
	s.payments[clone.ID] = &clone
	payment.UpdatedAt = clone.UpdatedAt
	return nil
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func TestRenewSubscriptionCheckoutUsesCustomerIdentity(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	userID := uuid.New()
	deviceID := uuid.New()
	subscriptionID := uuid.New()
	planID := uuid.New()

	var capturedBody string
	client := dexpay.NewClient("https://api.safephone.test", "pk_test_public", time.Second)
	client.SetHTTPClient(&http.Client{
		Timeout: time.Second,
		Transport: renewalRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			bodyBytes, _ := io.ReadAll(r.Body)
			capturedBody = string(bodyBytes)
			return &http.Response{
				StatusCode: http.StatusCreated,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"status":201,"message":"ok","data":{"reference":"SPAY_RENEW_ABC","amount":3500,"currency":"XOF","status":"pending","payment_url":"https://pay.example/checkout/SPAY_RENEW_ABC"}}`,
				)),
			}, nil
		}),
	})

	svc := &PaymentService{
		repo: &stubRenewalPaymentRepository{payments: map[uuid.UUID]*domain.Payment{}},
		subRepo: &stubRenewalSubscriptionRepository{
			subs: map[uuid.UUID]*domain.Subscription{
				subscriptionID: {
					ID:           subscriptionID,
					OrgID:        orgID,
					UserID:       userID,
					DeviceID:     deviceID,
					PlanID:       planID,
					Status:       domain.SubscriptionStatusExpired,
					BillingCycle: "monthly",
				},
			},
		},
		planRepo: &stubRenewalPlanRepository{
			plans: map[uuid.UUID]*domain.Plan{
				planID: {ID: planID, Slug: "essentiel", NameFR: "Essentiel", PriceMonthly: 3500, PriceAnnual: 35000},
			},
		},
		userRepo: &stubRenewalUserRepository{
			users: map[uuid.UUID]*domain.User{
				userID: {ID: userID, OrgID: orgID, FullName: "Checkout User", Email: "checkout@example.com"},
			},
		},
		deviceRepo: &stubRenewalDeviceRepository{
			devices: map[uuid.UUID]*domain.Device{
				deviceID: {ID: deviceID, OrgID: orgID, UserID: userID},
			},
		},
		dexpayClient:     client,
		frontendURL:      "https://app.safephone.test",
		backendPublicURL: "https://api.safephone.test",
	}

	_, appErr := svc.RenewSubscription(
		context.Background(),
		&auth.AuthContext{OrgID: orgID, UserID: userID},
		subscriptionID,
		planID,
		"monthly",
		nil,
	)
	if appErr != nil {
		t.Fatalf("expected no app error, got %#v", appErr)
	}
	if !strings.Contains(capturedBody, `"email":"checkout@example.com"`) {
		t.Fatalf("expected checkout payload to include user identity, got %q", capturedBody)
	}
}
