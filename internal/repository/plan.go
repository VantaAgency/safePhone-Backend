// Package repository implements data access for all domain entities.
package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cherif-safephone/safephone-backend/internal/domain"
)

// PlanRepository implements domain.PlanRepository using pgxpool.
type PlanRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// NewPlanRepository creates a new plan repository.
func NewPlanRepository(pool *pgxpool.Pool) *PlanRepository {
	return &PlanRepository{pool: pool, timeout: 5 * time.Second}
}

const planColumns = `id, slug, name_fr, name_en, price_monthly, price_annual, market, currency, tier,
       device_range_fr, device_range_en, features_fr, features_en,
       not_covered_fr, not_covered_en, service_time, is_popular, sort_order,
       max_smartphones, max_tablets, max_computers, max_game_consoles, max_tvs,
       claim_waiting_period_days,
       created_at, updated_at`

// List returns all plans ordered by sort_order.
func (r *PlanRepository) List(ctx context.Context) ([]domain.Plan, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `SELECT `+planColumns+` FROM plans ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []domain.Plan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, *p)
	}

	if plans == nil {
		plans = []domain.Plan{}
	}
	return plans, rows.Err()
}

// GetByID returns a plan by ID.
func (r *PlanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Plan, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	row := r.pool.QueryRow(ctx, `SELECT `+planColumns+` FROM plans WHERE id = $1`, id)
	return scanPlanRow(row)
}

// GetBySlug returns a plan by slug.
func (r *PlanRepository) GetBySlug(ctx context.Context, slug string) (*domain.Plan, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	row := r.pool.QueryRow(ctx, `SELECT `+planColumns+` FROM plans WHERE slug = $1`, slug)
	return scanPlanRow(row)
}

// EnsureDevelopmentTestPlan upserts the development-only payment test plan.
func (r *PlanRepository) EnsureDevelopmentTestPlan(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	deviceRangeFR := "Développement uniquement"
	deviceRangeEN := "Development only"
	featuresFR := []string{"Paiement DEXPAY", "Redirections", "Webhooks", "Activation de test"}
	featuresEN := []string{"DEXPAY payment", "Redirects", "Webhooks", "Test activation"}
	notCoveredFR := []string{"Usage commercial", "Production"}
	notCoveredEN := []string{"Commercial use", "Production"}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO plans (
			slug, name_fr, name_en, price_monthly, price_annual, tier,
			device_range_fr, device_range_en, features_fr, features_en,
			not_covered_fr, not_covered_en, service_time, is_popular, sort_order
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11::jsonb, $12::jsonb, $13, $14, $15)
		ON CONFLICT (slug) DO UPDATE
		SET
			name_fr = EXCLUDED.name_fr,
			name_en = EXCLUDED.name_en,
			price_monthly = EXCLUDED.price_monthly,
			price_annual = EXCLUDED.price_annual,
			tier = EXCLUDED.tier,
			device_range_fr = EXCLUDED.device_range_fr,
			device_range_en = EXCLUDED.device_range_en,
			features_fr = EXCLUDED.features_fr,
			features_en = EXCLUDED.features_en,
			not_covered_fr = EXCLUDED.not_covered_fr,
			not_covered_en = EXCLUDED.not_covered_en,
			service_time = EXCLUDED.service_time,
			is_popular = EXCLUDED.is_popular,
			sort_order = EXCLUDED.sort_order,
			updated_at = now()
	`,
		domain.DevelopmentTestPlanSlug,
		"Plan test",
		"Test Plan",
		100,
		100,
		"entry",
		deviceRangeFR,
		deviceRangeEN,
		mustJSON(featuresFR),
		mustJSON(featuresEN),
		mustJSON(notCoveredFR),
		mustJSON(notCoveredEN),
		"dev",
		false,
		999,
	)
	return err
}

func mustJSON(value any) []byte {
	payload, _ := json.Marshal(value)
	return payload
}

func scanPlan(rows pgx.Rows) (*domain.Plan, error) {
	var p domain.Plan
	var featuresFR, featuresEN, notCoveredFR, notCoveredEN json.RawMessage

	err := rows.Scan(
		&p.ID, &p.Slug, &p.NameFR, &p.NameEN, &p.PriceMonthly, &p.PriceAnnual, &p.Market, &p.Currency, &p.Tier,
		&p.DeviceRangeFR, &p.DeviceRangeEN, &featuresFR, &featuresEN,
		&notCoveredFR, &notCoveredEN, &p.ServiceTime, &p.IsPopular, &p.SortOrder,
		&p.MaxSmartphones, &p.MaxTablets, &p.MaxComputers, &p.MaxGameConsoles, &p.MaxTVs,
		&p.ClaimWaitingPeriodDays,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(featuresFR, &p.FeaturesFR)     //nolint:errcheck
	json.Unmarshal(featuresEN, &p.FeaturesEN)     //nolint:errcheck
	json.Unmarshal(notCoveredFR, &p.NotCoveredFR) //nolint:errcheck
	json.Unmarshal(notCoveredEN, &p.NotCoveredEN) //nolint:errcheck

	return &p, nil
}

func scanPlanRow(row pgx.Row) (*domain.Plan, error) {
	var p domain.Plan
	var featuresFR, featuresEN, notCoveredFR, notCoveredEN json.RawMessage

	err := row.Scan(
		&p.ID, &p.Slug, &p.NameFR, &p.NameEN, &p.PriceMonthly, &p.PriceAnnual, &p.Market, &p.Currency, &p.Tier,
		&p.DeviceRangeFR, &p.DeviceRangeEN, &featuresFR, &featuresEN,
		&notCoveredFR, &notCoveredEN, &p.ServiceTime, &p.IsPopular, &p.SortOrder,
		&p.MaxSmartphones, &p.MaxTablets, &p.MaxComputers, &p.MaxGameConsoles, &p.MaxTVs,
		&p.ClaimWaitingPeriodDays,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(featuresFR, &p.FeaturesFR)     //nolint:errcheck
	json.Unmarshal(featuresEN, &p.FeaturesEN)     //nolint:errcheck
	json.Unmarshal(notCoveredFR, &p.NotCoveredFR) //nolint:errcheck
	json.Unmarshal(notCoveredEN, &p.NotCoveredEN) //nolint:errcheck

	return &p, nil
}
