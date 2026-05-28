-- Seed two US plans for the United States market. Prices are stored as USD
-- cents (999 = $9.99, 1499 = $14.99) — the frontend formatter detects USD
-- plans by the `us_` slug prefix and formats accordingly.
--
-- The existing plans table schema (migration 8) has no `market` or
-- `currency` column. We rely on slug prefix as the market marker. A later
-- migration can add proper market/currency columns when we tackle the
-- backend Stripe integration (Phase 6 of the multi-market plan).

INSERT INTO plans (
  slug, name_fr, name_en, price_monthly, price_annual, tier,
  device_range_fr, device_range_en, features_fr, features_en,
  not_covered_fr, not_covered_en, service_time, is_popular, sort_order
) VALUES
  (
    'us_essential_monthly',
    'Essential',
    'Essential',
    999, 0,
    'essential',
    NULL, 'Smartphones',
    '["Cracked screen repair support", "Repair discounts at participating shops", "One approved claim per year", "Simple online claim submission", "Deductible may apply"]'::jsonb,
    '["Cracked screen repair support", "Repair discounts at participating shops", "One approved claim per year", "Simple online claim submission", "Deductible may apply"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage", "Pre-existing damage"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage", "Pre-existing damage"]'::jsonb,
    'standard', false, 100
  ),
  (
    'us_complete_monthly',
    'Complete',
    'Complete',
    1499, 0,
    'complete',
    NULL, 'Smartphones',
    '["Screen damage support", "Back glass repair support where eligible", "Liquid damage assessment support", "Up to two approved claims per year", "Priority claim review", "Deductible may apply"]'::jsonb,
    '["Screen damage support", "Back glass repair support where eligible", "Liquid damage assessment support", "Up to two approved claims per year", "Priority claim review", "Deductible may apply"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage", "Pre-existing damage"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage", "Pre-existing damage"]'::jsonb,
    'priority', true, 101
  )
ON CONFLICT (slug) DO NOTHING;
