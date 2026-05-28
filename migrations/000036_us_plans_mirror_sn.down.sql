DELETE FROM plans WHERE slug IN (
  'us_essentiel',
  'us_ecran_plus',
  'us_plus',
  'us_premium',
  'us_total'
);

-- Restore the previous 2-plan US seed so down/up cycle is symmetric.
INSERT INTO plans (
  slug, name_fr, name_en, price_monthly, price_annual, tier,
  device_range_fr, device_range_en, features_fr, features_en,
  not_covered_fr, not_covered_en, service_time, is_popular, sort_order
) VALUES
  (
    'us_essential_monthly', 'Essential', 'Essential', 999, 9990, 'essential',
    NULL, 'Smartphones',
    '["Cracked screen repair support"]'::jsonb,
    '["Cracked screen repair support"]'::jsonb,
    '["Theft or loss"]'::jsonb,
    '["Theft or loss"]'::jsonb,
    'standard', false, 100
  ),
  (
    'us_complete_monthly', 'Complete', 'Complete', 1499, 14990, 'complete',
    NULL, 'Smartphones',
    '["Screen damage support"]'::jsonb,
    '["Screen damage support"]'::jsonb,
    '["Theft or loss"]'::jsonb,
    '["Theft or loss"]'::jsonb,
    'priority', true, 101
  )
ON CONFLICT (slug) DO NOTHING;
