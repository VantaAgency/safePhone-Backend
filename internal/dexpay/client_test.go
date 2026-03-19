package dexpay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestClientCreateCheckoutSessionSendsDocumentedHeaders(t *testing.T) {
	t.Parallel()

	var gotAPIKey string
	var gotContentType string
	var gotPath string

	client := NewClient("https://api-sandbox.dexpay.africa/api/v1", "pk_test_public", time.Second)
	client.httpClient = &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotAPIKey = r.Header.Get("x-api-key")
			gotContentType = r.Header.Get("Content-Type")
			gotPath = r.URL.Path

			if _, err := io.ReadAll(r.Body); err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}

			return &http.Response{
				StatusCode: http.StatusCreated,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"status":201,"message":"Checkout session created successfully","data":{"reference":"SPAY_123","amount":3500,"currency":"XOF","status":"pending","payment_url":"https://pay.dexpay.africa/checkout/SPAY_123","sandbox_payment_url":"https://sandbox.dexpay.africa/checkout/SPAY_123","isSandbox":true}}`,
				)),
			}, nil
		}),
	}

	session, err := client.CreateCheckoutSession(context.Background(), CreateCheckoutSessionRequest{
		Reference:  "SPAY_123",
		ItemName:   "SafePhone Essentiel",
		Amount:     3500,
		Currency:   "XOF",
		CountryISO: "SN",
		WebhookURL: "https://api.safephone.example/api/v1/webhooks/dexpay",
		SuccessURL: "https://safephone.example/paiement/succes",
		FailureURL: "https://safephone.example/paiement/echec",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if session == nil || session.Reference != "SPAY_123" {
		t.Fatalf("expected decoded checkout session response, got %#v", session)
	}
	if gotPath != "/api/v1/checkout-sessions" {
		t.Fatalf("expected request path /api/v1/checkout-sessions, got %q", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", gotContentType)
	}
	if gotAPIKey != "pk_test_public" {
		t.Fatalf("expected x-api-key header to be sent, got %q", gotAPIKey)
	}
}

func TestClientGetCheckoutSessionUsesReferenceEndpoint(t *testing.T) {
	t.Parallel()

	var gotPath string

	client := NewClient("https://api-sandbox.dexpay.africa/api/v1", "pk_test_public", time.Second)
	client.httpClient = &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotPath = r.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"reference":"SPAY_123","amount":3500,"currency":"XOF","status":"completed","payment_url":"https://pay.dexpay.africa/checkout/SPAY_123","isSandbox":false}`,
				)),
			}, nil
		}),
	}

	session, err := client.GetCheckoutSession(context.Background(), "SPAY_123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if session == nil || session.Status != "completed" {
		t.Fatalf("expected decoded session response, got %#v", session)
	}
	if gotPath != "/api/v1/checkout-sessions/SPAY_123" {
		t.Fatalf("expected reference path, got %q", gotPath)
	}
}

func TestClientCheckoutSessionAuthErrorIsDetailedButSafe(t *testing.T) {
	t.Parallel()

	client := NewClient("https://api-sandbox.dexpay.africa/api/v1", "pk_test_public", time.Second)
	client.httpClient = &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body: io.NopCloser(strings.NewReader(
					`{"status":401,"message":"Unauthorized"}`,
				)),
			}, nil
		}),
	}

	_, err := client.CreateCheckoutSession(context.Background(), CreateCheckoutSessionRequest{
		Reference:  "SPAY_123",
		ItemName:   "SafePhone Essentiel",
		Amount:     3500,
		Currency:   "XOF",
		CountryISO: "SN",
		WebhookURL: "https://api.safephone.example/api/v1/webhooks/dexpay",
		SuccessURL: "https://safephone.example/paiement/succes",
		FailureURL: "https://safephone.example/paiement/echec",
	})
	if err == nil {
		t.Fatal("expected an auth error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", apiErr.StatusCode)
	}
	if strings.Contains(apiErr.Error(), "pk_test_public") {
		t.Fatalf("expected API key to stay out of the error message, got %q", apiErr.Error())
	}
}
