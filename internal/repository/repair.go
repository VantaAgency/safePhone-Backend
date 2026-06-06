package repository

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

const repairColumns = `id, org_id, user_id, reference, device_brand, device_model, repair_type,
       service_mode, center_id, preferred_date, preferred_time, scheduled_date, scheduled_time,
       customer_name, customer_phone, customer_phone_normalized, status, repair_amount_minor,
       market, request_source, created_at, updated_at`

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

	orgID := booking.OrgID
	if orgID == nil {
		sharedOrgID, err := r.getSharedOrgID(ctx)
		if err != nil {
			return err
		}
		orgID = &sharedOrgID
	}

	// Try up to 5 times to get a unique reference (collision is extremely unlikely)
	for range 5 {
		ref := generateReference()
		booking.Reference = ref
		market := booking.Market
		if market == "" {
			market = domain.MarketSN
		}
		created, err := scanRepairBooking(r.pool.QueryRow(ctx, `
			INSERT INTO repair_bookings
				(org_id, user_id, reference, device_brand, device_model, repair_type, service_mode,
				 center_id, preferred_date, preferred_time, customer_name, customer_phone,
				 customer_phone_normalized, status, market, request_source)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			RETURNING `+repairColumns+`
		`,
			orgID, booking.UserID, ref,
			booking.DeviceBrand, booking.DeviceModel, booking.RepairType,
			booking.ServiceMode, booking.CenterID,
			booking.PreferredDate, booking.PreferredTime,
			booking.CustomerName, booking.CustomerPhone,
			booking.CustomerPhoneNormalized, booking.Status, market, booking.RequestSource,
		))
		if err == nil {
			if created != nil {
				*booking = *created
			}
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

// GetByID returns a repair request by ID.
func (r *RepairRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.RepairBooking, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return scanRepairBooking(r.pool.QueryRow(ctx, `SELECT `+repairColumns+` FROM repair_bookings WHERE id = $1`, id))
}

// GetByReferenceAndPhone returns a repair request for public tracking.
func (r *RepairRepository) GetByReferenceAndPhone(ctx context.Context, reference, normalizedPhone string) (*domain.RepairBooking, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	return scanRepairBooking(r.pool.QueryRow(ctx, `
		SELECT `+repairColumns+`
		FROM repair_bookings
		WHERE upper(reference) = upper($1) AND customer_phone_normalized = $2
	`, strings.TrimSpace(reference), normalizedPhone))
}

// ListByOrgAndUser returns repair requests for a specific authenticated user.
func (r *RepairRepository) ListByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]domain.RepairBooking, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT `+repairColumns+`
		FROM repair_bookings
		WHERE org_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, orgID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRepairBookings(rows)
}

// ListByOrg returns repair requests in an org with optional status and search filters.
func (r *RepairRepository) ListByOrg(ctx context.Context, orgID uuid.UUID, status *string, search, market string, limit, offset int) ([]domain.RepairBooking, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	statusFilter := ""
	if status != nil {
		statusFilter = *status
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+repairColumns+`
		FROM repair_bookings
		WHERE org_id = $1
		  AND ($2 = '' OR status = $2)
		  AND (
			$3 = ''
			OR lower(reference) LIKE '%' || lower($3) || '%'
			OR lower(customer_name) LIKE '%' || lower($3) || '%'
			OR lower(customer_phone) LIKE '%' || lower($3) || '%'
			OR lower(device_brand) LIKE '%' || lower($3) || '%'
			OR lower(device_model) LIKE '%' || lower($3) || '%'
			OR lower(repair_type) LIKE '%' || lower($3) || '%'
		  )
		  AND ($4 = '' OR market = $4)
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6
	`, orgID, statusFilter, strings.TrimSpace(search), market, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRepairBookings(rows)
}

// Update persists admin edits to a repair request.
func (r *RepairRepository) Update(ctx context.Context, booking *domain.RepairBooking) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err := r.pool.Exec(ctx, `
		UPDATE repair_bookings
		SET
			status = $2,
			scheduled_date = $3,
			scheduled_time = $4,
			repair_amount_minor = $5,
			updated_at = now()
		WHERE id = $1
	`, booking.ID, booking.Status, booking.ScheduledDate, booking.ScheduledTime, booking.RepairAmountMinor)
	return err
}

func (r *RepairRepository) getSharedOrgID(ctx context.Context) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM organizations
		WHERE slug = 'safephone'
		LIMIT 1
	`).Scan(&orgID)
	return orgID, err
}

func scanRepairBookings(rows pgx.Rows) ([]domain.RepairBooking, error) {
	var bookings []domain.RepairBooking
	for rows.Next() {
		booking, err := scanRepairBooking(rows)
		if err != nil {
			return nil, err
		}
		if booking != nil {
			bookings = append(bookings, *booking)
		}
	}
	if bookings == nil {
		bookings = []domain.RepairBooking{}
	}
	return bookings, rows.Err()
}

func scanRepairBooking(scanner interface{ Scan(dest ...any) error }) (*domain.RepairBooking, error) {
	var booking domain.RepairBooking
	var preferredDate time.Time
	var scheduledDate *time.Time
	err := scanner.Scan(
		&booking.ID,
		&booking.OrgID,
		&booking.UserID,
		&booking.Reference,
		&booking.DeviceBrand,
		&booking.DeviceModel,
		&booking.RepairType,
		&booking.ServiceMode,
		&booking.CenterID,
		&preferredDate,
		&booking.PreferredTime,
		&scheduledDate,
		&booking.ScheduledTime,
		&booking.CustomerName,
		&booking.CustomerPhone,
		&booking.CustomerPhoneNormalized,
		&booking.Status,
		&booking.RepairAmountMinor,
		&booking.Market,
		&booking.RequestSource,
		&booking.CreatedAt,
		&booking.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	booking.PreferredDate = preferredDate.Format("2006-01-02")
	if scheduledDate != nil {
		formatted := scheduledDate.Format("2006-01-02")
		booking.ScheduledDate = &formatted
	}

	return &booking, nil
}
