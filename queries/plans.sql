-- name: ListPlans :many
SELECT id, slug, name_fr, name_en, price_monthly, price_annual, tier,
       device_range_fr, device_range_en, features_fr, features_en,
       not_covered_fr, not_covered_en, service_time, is_popular, sort_order,
       created_at, updated_at
FROM plans
ORDER BY sort_order ASC;

-- name: GetPlanByID :one
SELECT id, slug, name_fr, name_en, price_monthly, price_annual, tier,
       device_range_fr, device_range_en, features_fr, features_en,
       not_covered_fr, not_covered_en, service_time, is_popular, sort_order,
       created_at, updated_at
FROM plans
WHERE id = $1;

-- name: GetPlanBySlug :one
SELECT id, slug, name_fr, name_en, price_monthly, price_annual, tier,
       device_range_fr, device_range_en, features_fr, features_en,
       not_covered_fr, not_covered_en, service_time, is_popular, sort_order,
       created_at, updated_at
FROM plans
WHERE slug = $1;
