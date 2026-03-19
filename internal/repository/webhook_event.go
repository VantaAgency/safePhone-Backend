package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// WebhookEventRepository implements domain.WebhookEventRepository.
type WebhookEventRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewWebhookEventRepository creates a new webhook event repository.
func NewWebhookEventRepository(pool *pgxpool.Pool) *WebhookEventRepository {
	return &WebhookEventRepository{pool: pool, timeout: 5 * time.Second}
}

// Exists checks if a webhook event with the given idempotency key has already been processed.
func (r *WebhookEventRepository) Exists(ctx context.Context, idempotencyKey string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhook_events WHERE idempotency_key = $1)`, idempotencyKey).Scan(&exists)
	return exists, err
}

// Create records a processed webhook event for idempotency tracking.
func (r *WebhookEventRepository) Create(ctx context.Context, event *domain.WebhookEvent) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_events (provider, event_type, provider_ref, idempotency_key, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, event.Provider, event.EventType, event.ProviderRef, event.IdempotencyKey, event.Payload)
	return err
}
