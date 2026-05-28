-- 1) Annual prices for the 2 existing US plans (10 monthly payments = ~17% off,
--    matching the "save up to 15%" toggle claim).
UPDATE plans SET price_annual = 9990  WHERE slug = 'us_essential_monthly';
UPDATE plans SET price_annual = 14990 WHERE slug = 'us_complete_monthly';

-- 2) Third US plan tier for richer pricing on the homepage (3 cards instead of 2).
INSERT INTO plans (
  slug, name_fr, name_en, price_monthly, price_annual, tier,
  device_range_fr, device_range_en, features_fr, features_en,
  not_covered_fr, not_covered_en, service_time, is_popular, sort_order
) VALUES (
  'us_plus_monthly',
  'Plus',
  'Plus',
  1999, 19990,
  'plus',
  NULL, 'Smartphones',
  '["Screen damage support", "Back glass repair support where eligible", "Liquid damage assessment support", "Up to three approved claims per year", "Priority claim review within 24h", "Loaner device when available", "Lower deductible"]'::jsonb,
  '["Screen damage support", "Back glass repair support where eligible", "Liquid damage assessment support", "Up to three approved claims per year", "Priority claim review within 24h", "Loaner device when available", "Lower deductible"]'::jsonb,
  '["Theft or loss", "Cosmetic-only damage", "Pre-existing damage"]'::jsonb,
  '["Theft or loss", "Cosmetic-only damage", "Pre-existing damage"]'::jsonb,
  'priority', false, 102
)
ON CONFLICT (slug) DO NOTHING;
