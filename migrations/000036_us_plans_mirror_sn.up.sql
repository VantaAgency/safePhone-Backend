-- Replace the 3 previously seeded US plans with 5 plans that mirror the SN
-- structure 1:1 — same tiers, same coverage progression, just USD pricing
-- and US-market wording (no insurance/warranty/guaranteed language).
--
-- SN tier → US slug mapping:
--   essentiel    → us_essentiel    ($9.99/mo)
--   ecran-plus   → us_ecran_plus   ($14.99/mo)
--   plus         → us_plus         ($19.99/mo)
--   haute        → us_premium      ($29.99/mo)
--   totale       → us_total        ($44.99/mo)
--
-- Annual prices = 10× monthly (~17% off, matches "save 15%" toggle badge).

DELETE FROM plans WHERE slug IN (
  'us_essential_monthly',
  'us_complete_monthly',
  'us_plus_monthly'
);

INSERT INTO plans (
  slug, name_fr, name_en, price_monthly, price_annual, tier,
  device_range_fr, device_range_en, features_fr, features_en,
  not_covered_fr, not_covered_en, service_time, is_popular, sort_order
) VALUES
  (
    'us_essentiel',
    'Essential',
    'Essential',
    999, 9990,
    'entry',
    NULL, 'Smartphones up to $400',
    '["Cracked screen repair support", "Repair discounts at participating shops", "One approved claim per year", "Online claim submission", "Deductible may apply"]'::jsonb,
    '["Cracked screen repair support", "Repair discounts at participating shops", "One approved claim per year", "Online claim submission", "Deductible may apply"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage", "Pre-existing damage"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage", "Pre-existing damage"]'::jsonb,
    '72h',
    false,
    100
  ),
  (
    'us_ecran_plus',
    'Screen+',
    'Screen+',
    1499, 14990,
    'mid',
    NULL, 'Smartphones up to $800',
    '["Screen damage support (front + back glass)", "Repair discounts", "Two approved claims per year", "Priority claim review", "Online claim submission", "Deductible may apply"]'::jsonb,
    '["Screen damage support (front + back glass)", "Repair discounts", "Two approved claims per year", "Priority claim review", "Online claim submission", "Deductible may apply"]'::jsonb,
    '["Theft or loss", "Full liquid immersion", "Cosmetic-only damage"]'::jsonb,
    '["Theft or loss", "Full liquid immersion", "Cosmetic-only damage"]'::jsonb,
    '48h',
    true,
    101
  ),
  (
    'us_plus',
    'Plus',
    'Plus',
    1999, 19990,
    'mid-high',
    NULL, 'Smartphones up to $1,200',
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Up to three approved claims per year", "Faster handling"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Up to three approved claims per year", "Faster handling"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage"]'::jsonb,
    '48h',
    false,
    102
  ),
  (
    'us_premium',
    'Premium',
    'Premium',
    2999, 29990,
    'premium',
    NULL, 'Smartphones up to $1,800',
    '["Screen damage support", "Back glass repair", "Liquid damage support", "Hardware failure support", "Up to four approved claims per year", "Priority claim review within 24h", "Loaner device when available", "Lower deductible"]'::jsonb,
    '["Screen damage support", "Back glass repair", "Liquid damage support", "Hardware failure support", "Up to four approved claims per year", "Priority claim review within 24h", "Loaner device when available", "Lower deductible"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage"]'::jsonb,
    '["Theft or loss", "Cosmetic-only damage"]'::jsonb,
    '24h',
    false,
    103
  ),
  (
    'us_total',
    'Total',
    'Total',
    4499, 44990,
    'household',
    NULL, 'Up to 3 smartphones per household',
    '["Cover up to 3 household phones", "Screen damage support", "Back glass repair", "Liquid damage support", "Hardware failure support", "Up to six approved claims per year", "Priority claim review within 12h", "Loaner device", "Dedicated member support"]'::jsonb,
    '["Cover up to 3 household phones", "Screen damage support", "Back glass repair", "Liquid damage support", "Hardware failure support", "Up to six approved claims per year", "Priority claim review within 12h", "Loaner device", "Dedicated member support"]'::jsonb,
    '["Pre-existing damage", "Cosmetic-only damage"]'::jsonb,
    '["Pre-existing damage", "Cosmetic-only damage"]'::jsonb,
    '12h',
    false,
    104
  )
ON CONFLICT (slug) DO NOTHING;
