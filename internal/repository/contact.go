package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// ContactRepository implements domain.ContactRepository using pgxpool.
type ContactRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewContactRepository creates a new contact repository.
func NewContactRepository(pool *pgxpool.Pool) *ContactRepository {
	return &ContactRepository{pool: pool, timeout: 5 * time.Second}
}

// Create inserts a new contact message.
func (r *ContactRepository) Create(ctx context.Context, msg *domain.ContactMessage) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return r.pool.QueryRow(ctx, `
		INSERT INTO contact_messages (name, email, subject, message)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`, msg.Name, msg.Email, msg.Subject, msg.Message).Scan(&msg.ID, &msg.CreatedAt)
}
