package repository

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// RepairRepository implements domain.RepairRepository using pgxpool.
type RepairRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewRepairRepository creates a new repair repository.
func NewRepairRepository(pool *pgxpool.Pool) *RepairRepository {
	return &RepairRepository{pool: pool, timeout: 5 * time.Second}
}

// generateReference creates a unique MBT-XXXXXX reference.
func generateReference() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("MBT-%s", string(b))
}

// Create inserts a new repair booking, generating a unique reference.
func (r *RepairRepository) Create(ctx context.Context, booking *domain.RepairBooking) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Try up to 5 times to get a unique reference (collision is extremely unlikely)
	for range 5 {
		ref := generateReference()
		err := r.pool.QueryRow(ctx, `
			INSERT INTO repair_bookings
				(org_id, user_id, reference, device_type, repair_type, location_id,
				 booking_date, booking_time, customer_name, customer_phone, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'confirmed')
			RETURNING id, reference, created_at, updated_at
		`,
			booking.OrgID, booking.UserID, ref,
			booking.DeviceType, booking.RepairType, booking.LocationID,
			booking.BookingDate, booking.BookingTime,
			booking.CustomerName, booking.CustomerPhone,
		).Scan(&booking.ID, &booking.Reference, &booking.CreatedAt, &booking.UpdatedAt)
		if err == nil {
			booking.Status = "confirmed"
			return nil
		}
		// If unique constraint violation, retry with a new reference
		if isUniqueViolation(err) {
			continue
		}
		return err
	}
	return fmt.Errorf("failed to generate unique repair reference after 5 attempts")
}
