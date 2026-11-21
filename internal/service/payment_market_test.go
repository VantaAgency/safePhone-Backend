package service

import (
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

func TestDexpayReturnsToSenegal(t *testing.T) {
	svc := PaymentService{
		frontendURL:      "https://www.safephone.sn",
		backendPublicURL: "https://backend.example.test",
	}
	payment := domain.Payment{ID: uuid.New(), Currency: "XOF"}
	req := svc.buildCheckoutSessionRequest(&domain.User{}, &domain.Plan{}, &domain.Subscription{}, &payment)
	for path, raw := range map[string]string{
		"/sn/paiement/succes": req.SuccessURL,
		"/sn/paiement/echec":  req.FailureURL,
	} {
		u, err := url.Parse(raw)
		if err != nil || u.Host != "www.safephone.sn" || u.Path != path ||
			u.Query().Get("payment_id") != payment.ID.String() {
			t.Fatalf("unexpected DEXPAY callback %q: %v", raw, err)
		}
	}
	if req.Currency != "XOF" || req.CountryISO != "SN" {
		t.Fatalf("unexpected checkout market %q %q", req.Currency, req.CountryISO)
	}
	if err := validateDexpayPlan(&domain.Plan{Market: domain.MarketUS, Currency: domain.CurrencyUSD}, false); err == nil {
		t.Fatal("DEXPAY accepted a US plan")
	}
}
