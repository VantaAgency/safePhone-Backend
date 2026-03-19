// Package dexpay provides a client for the DEXPAY payment gateway API.
package dexpay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Client communicates with the DEXPAY API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// APIError captures a structured DEXPAY API failure.
type APIError struct {
	StatusCode  int
	Method      string
	Path        string
	Environment string
	Message     string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf(
		"dexpay API error %d for %s %s (environment=%s): %s",
		e.StatusCode,
		e.Method,
		e.Path,
		e.Environment,
		e.Message,
	)
}

// CreateCheckoutSessionRequest is the payload for POST /checkout-sessions.
type CreateCheckoutSessionRequest struct {
	Reference  string                   `json:"reference"`
	ItemName   string                   `json:"item_name"`
	Amount     int                      `json:"amount"`
	Currency   string                   `json:"currency"`
	CountryISO string                   `json:"countryISO"`
	WebhookURL string                   `json:"webhook_url"`
	SuccessURL string                   `json:"success_url"`
	FailureURL string                   `json:"failure_url"`
	Customer   *CheckoutSessionCustomer `json:"customer,omitempty"`
}

// CheckoutSessionCustomer identifies the checkout customer.
type CheckoutSessionCustomer struct {
	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// CheckoutSession represents a DEXPAY checkout session.
type CheckoutSession struct {
	Reference         string                   `json:"reference"`
	MerchantID        string                   `json:"merchant_id,omitempty"`
	ItemName          string                   `json:"item_name,omitempty"`
	Amount            int                      `json:"amount"`
	Currency          string                   `json:"currency"`
	CountryISO        string                   `json:"countryISO,omitempty"`
	Status            string                   `json:"status"`
	WebhookURL        string                   `json:"webhook_url,omitempty"`
	SuccessURL        string                   `json:"success_url,omitempty"`
	FailureURL        string                   `json:"failure_url,omitempty"`
	PaymentURL        string                   `json:"payment_url"`
	SandboxPaymentURL string                   `json:"sandbox_payment_url,omitempty"`
	Customer          *CheckoutSessionCustomer `json:"customer,omitempty"`
	IsSandbox         bool                     `json:"isSandbox"`
	ExpiresAt         string                   `json:"expires_at,omitempty"`
	CreatedAt         string                   `json:"createdAt,omitempty"`
	UpdatedAt         string                   `json:"updatedAt,omitempty"`
}

type apiResponse[T any] struct {
	Status  int    `json:"status"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// NewClient creates a DEXPAY API client.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Environment returns the resolved DEXPAY environment for this client.
func (c *Client) Environment() string {
	return dexpayEnvironmentFromBaseURL(c.baseURL)
}

// IsSandbox reports whether the client targets the DEXPAY sandbox API.
func (c *Client) IsSandbox() bool {
	return c.Environment() == "sandbox"
}

// CreateCheckoutSession creates a hosted DEXPAY checkout session.
func (c *Client) CreateCheckoutSession(ctx context.Context, req CreateCheckoutSessionRequest) (*CheckoutSession, error) {
	var resp apiResponse[CheckoutSession]
	if err := c.do(ctx, http.MethodPost, "/checkout-sessions", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetCheckoutSession retrieves a hosted DEXPAY checkout session by reference.
func (c *Client) GetCheckoutSession(ctx context.Context, reference string) (*CheckoutSession, error) {
	var session CheckoutSession
	if err := c.do(ctx, http.MethodGet, "/checkout-sessions/"+reference, nil, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// do executes an HTTP request against the DEXPAY API.
func (c *Client) do(ctx context.Context, method, path string, body any, target any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}

	slog.Debug("dexpay request",
		"method", method,
		"path", path,
		"environment", c.Environment(),
		"base_url", c.baseURL,
		"api_key_present", c.apiKey != "",
		"api_key_type", dexpayCredentialType(c.apiKey),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dexpay request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read dexpay response: %w", err)
	}

	if resp.StatusCode >= 400 {
		responseBody := summarizeForLog(respBody)
		logArgs := []any{
			"method", method,
			"path", path,
			"status_code", resp.StatusCode,
			"environment", c.Environment(),
			"base_url", c.baseURL,
			"api_key_present", c.apiKey != "",
			"api_key_type", dexpayCredentialType(c.apiKey),
			"response_body", responseBody,
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			slog.Error("dexpay authentication failed", logArgs...)
		} else {
			slog.Error("dexpay API request failed", logArgs...)
		}
		return &APIError{
			StatusCode:  resp.StatusCode,
			Method:      method,
			Path:        path,
			Environment: c.Environment(),
			Message:     responseBody,
		}
	}

	if target != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, target); err != nil {
			return fmt.Errorf("decode dexpay response: %w", err)
		}
	}
	return nil
}

func dexpayEnvironmentFromBaseURL(baseURL string) string {
	switch strings.TrimRight(strings.TrimSpace(baseURL), "/") {
	case "https://api-sandbox.dexpay.africa/api/v1":
		return "sandbox"
	case "https://api.dexpay.africa/api/v1":
		return "production"
	default:
		return "custom"
	}
}

func dexpayCredentialType(value string) string {
	switch {
	case strings.HasPrefix(value, "pk_test_"), strings.HasPrefix(value, "sk_test_"):
		return "test"
	case strings.HasPrefix(value, "pk_live_"), strings.HasPrefix(value, "sk_live_"):
		return "live"
	case value == "":
		return "missing"
	default:
		return "unknown"
	}
}

func summarizeForLog(body []byte) string {
	summary := strings.Join(strings.Fields(string(body)), " ")
	const maxLen = 240
	if len(summary) > maxLen {
		return summary[:maxLen] + "..."
	}
	return summary
}
