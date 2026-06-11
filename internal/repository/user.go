package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// UserRepository implements domain.UserRepository using pgxpool.
type UserRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewUserRepository creates a new user repository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool, timeout: 5 * time.Second}
}

// GetByID returns a user by their SafePhone ID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var u domain.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, email, full_name, phone, role, market, better_auth_id,
		       created_at, updated_at, deleted_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&u.ID, &u.OrgID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.Market, &u.BetterAuthID,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

// GetStripeCustomerID returns the user's Stripe customer ID, or empty
// string when not set.
func (r *UserRepository) GetStripeCustomerID(ctx context.Context, userID uuid.UUID) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var customerID *string
	err := r.pool.QueryRow(ctx, `
		SELECT stripe_customer_id FROM users WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&customerID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if customerID == nil {
		return "", nil
	}
	return *customerID, nil
}

// SetStripeCustomerID persists the Stripe customer ID on a user.
func (r *UserRepository) SetStripeCustomerID(ctx context.Context, userID uuid.UUID, customerID string) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE users SET stripe_customer_id = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, userID, customerID)
	return err
}

// GetByStripeCustomerID returns the user owning the given Stripe customer
// ID, or nil when no match.
func (r *UserRepository) GetByStripeCustomerID(ctx context.Context, customerID string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var u domain.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, org_id, email, full_name, phone, role, market, better_auth_id,
		       created_at, updated_at, deleted_at
		FROM users
		WHERE stripe_customer_id = $1 AND deleted_at IS NULL
		LIMIT 1
	`, customerID).Scan(&u.ID, &u.OrgID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.Market, &u.BetterAuthID,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

// Update persists changes to a user record.
func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE users SET phone = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, u.ID, u.Phone)
	return err
}

// UpdateRole updates the role on both the users table and the Better Auth "user" table.
func (r *UserRepository) UpdateRole(ctx context.Context, userID uuid.UUID, role string) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Update SafePhone users table
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET role = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, userID, role)
	if err != nil {
		return err
	}

	// Update Better Auth "user" table via better_auth_id
	_, err = r.pool.Exec(ctx, `
		UPDATE "user" SET role = $2, "updatedAt" = now()
		WHERE id = (SELECT better_auth_id FROM users WHERE id = $1 AND deleted_at IS NULL)
	`, userID, role)
	return err
}

// GetEmployeeProfile returns the employee profile for a SafePhone user.
func (r *UserRepository) GetEmployeeProfile(ctx context.Context, orgID, userID uuid.UUID) (*domain.EmployeeProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var profile domain.EmployeeProfile
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, org_id, status, suspended_reason, created_by, updated_by, created_at, updated_at
		FROM employee_profiles
		WHERE org_id = $1 AND user_id = $2
	`, orgID, userID).Scan(
		&profile.UserID,
		&profile.OrgID,
		&profile.Status,
		&profile.SuspendedReason,
		&profile.CreatedBy,
		&profile.UpdatedBy,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &profile, nil
}
