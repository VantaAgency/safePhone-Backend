-- Plan content v2 — single set of 5 standard services + per-plan device
-- coverage matrix. SN keeps XOF prices; US essentiel drops from $9.99 to
-- $7.99 (799 cents), other US prices unchanged.
--
-- INSERT … ON CONFLICT (slug) DO UPDATE preserves the existing row id,
-- which is critical because subscriptions reference plan_id by FK.

-- The 5 services that every production plan now provides, in canonical
-- order. Stored verbatim as JSONB; the per-card "30-day waiting period"
-- notice is rendered separately (it's universal, not per-plan).

-- ── SN plans ──────────────────────────────────────────────────────────────

INSERT INTO plans (
  slug, name_fr, name_en, price_monthly, price_annual, tier,
  device_range_fr, device_range_en,
  features_fr, features_en,
  not_covered_fr, not_covered_en,
  service_time, is_popular, sort_order,
  market, currency,
  max_smartphones, max_tablets, max_computers, max_game_consoles, max_tvs,
  claim_waiting_period_days
) VALUES
  (
    'essentiel', 'Essentiel', 'Essential',
    15000, 150000, 'entry',
    '1 smartphone', '1 smartphone',
    '["Support écran cassé", "Support vitre arrière", "Évaluation dégâts liquides", "Support panne matérielle", "Support batterie endommagée"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '48h', false, 100,
    'SN', 'XOF',
    1, 0, 0, 0, 0,
    30
  ),
  (
    'ecran-plus', 'Écran+', 'Screen+',
    35000, 350000, 'mid',
    '1 smartphone + 1 tablette', '1 smartphone + 1 tablet',
    '["Support écran cassé", "Support vitre arrière", "Évaluation dégâts liquides", "Support panne matérielle", "Support batterie endommagée"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '48h', true, 101,
    'SN', 'XOF',
    1, 1, 0, 0, 0,
    30
  ),
  (
    'plus', 'Plus', 'Plus',
    60000, 600000, 'mid-high',
    '2 smartphones + 1 tablette + 1 console', '2 smartphones + 1 tablet + 1 game console',
    '["Support écran cassé", "Support vitre arrière", "Évaluation dégâts liquides", "Support panne matérielle", "Support batterie endommagée"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '48h', false, 102,
    'SN', 'XOF',
    2, 1, 0, 1, 0,
    30
  ),
  (
    'haute', 'Haute', 'Haute',
    100000, 1000000, 'premium',
    '3 smartphones + 2 tablettes + 1 PC + 1 console', '3 smartphones + 2 tablets + 1 PC + 1 game console',
    '["Support écran cassé", "Support vitre arrière", "Évaluation dégâts liquides", "Support panne matérielle", "Support batterie endommagée"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '24h', false, 103,
    'SN', 'XOF',
    3, 2, 1, 1, 0,
    30
  ),
  (
    'totale', 'Totale', 'Total',
    150000, 1500000, 'household',
    '4 smartphones + 3 tablettes + 2 PC + 2 consoles + 1 TV', '4 smartphones + 3 tablets + 2 PCs + 2 game consoles + 1 TV',
    '["Support écran cassé", "Support vitre arrière", "Évaluation dégâts liquides", "Support panne matérielle", "Support batterie endommagée"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '24h', false, 104,
    'SN', 'XOF',
    4, 3, 2, 2, 1,
    30
  )
ON CONFLICT (slug) DO UPDATE SET
  name_fr                   = EXCLUDED.name_fr,
  name_en                   = EXCLUDED.name_en,
  price_monthly             = EXCLUDED.price_monthly,
  price_annual              = EXCLUDED.price_annual,
  tier                      = EXCLUDED.tier,
  device_range_fr           = EXCLUDED.device_range_fr,
  device_range_en           = EXCLUDED.device_range_en,
  features_fr               = EXCLUDED.features_fr,
  features_en               = EXCLUDED.features_en,
  not_covered_fr            = EXCLUDED.not_covered_fr,
  not_covered_en            = EXCLUDED.not_covered_en,
  service_time              = EXCLUDED.service_time,
  is_popular                = EXCLUDED.is_popular,
  sort_order                = EXCLUDED.sort_order,
  market                    = EXCLUDED.market,
  currency                  = EXCLUDED.currency,
  max_smartphones           = EXCLUDED.max_smartphones,
  max_tablets               = EXCLUDED.max_tablets,
  max_computers             = EXCLUDED.max_computers,
  max_game_consoles         = EXCLUDED.max_game_consoles,
  max_tvs                   = EXCLUDED.max_tvs,
  claim_waiting_period_days = EXCLUDED.claim_waiting_period_days,
  updated_at                = now();

-- ── US plans ──────────────────────────────────────────────────────────────
-- us_essentiel drops to $7.99/mo, annual = 10× monthly to keep the
-- "save 17%" math (matches existing SN pattern).

INSERT INTO plans (
  slug, name_fr, name_en, price_monthly, price_annual, tier,
  device_range_fr, device_range_en,
  features_fr, features_en,
  not_covered_fr, not_covered_en,
  service_time, is_popular, sort_order,
  market, currency,
  max_smartphones, max_tablets, max_computers, max_game_consoles, max_tvs,
  claim_waiting_period_days
) VALUES
  (
    'us_essentiel', 'Essential', 'Essential',
    799, 7990, 'entry',
    NULL, '1 smartphone',
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '72h', false, 100,
    'US', 'USD',
    1, 0, 0, 0, 0,
    30
  ),
  (
    'us_ecran_plus', 'Screen+', 'Screen+',
    1499, 14990, 'mid',
    NULL, '1 smartphone + 1 tablet',
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '48h', true, 101,
    'US', 'USD',
    1, 1, 0, 0, 0,
    30
  ),
  (
    'us_plus', 'Plus', 'Plus',
    1999, 19990, 'mid-high',
    NULL, '2 smartphones + 1 tablet + 1 game console',
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '48h', false, 102,
    'US', 'USD',
    2, 1, 0, 1, 0,
    30
  ),
  (
    'us_premium', 'Premium', 'Premium',
    2999, 29990, 'premium',
    NULL, '3 smartphones + 2 tablets + 1 PC + 1 game console',
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '24h', false, 103,
    'US', 'USD',
    3, 2, 1, 1, 0,
    30
  ),
  (
    'us_total', 'Total', 'Total',
    4499, 44990, 'household',
    NULL, '4 smartphones + 3 tablets + 2 PCs + 2 game consoles + 1 TV',
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '["Screen damage support", "Back glass repair support", "Liquid damage assessment", "Hardware failure support", "Battery damage support"]'::jsonb,
    '[]'::jsonb, '[]'::jsonb,
    '12h', false, 104,
    'US', 'USD',
    4, 3, 2, 2, 1,
    30
  )
ON CONFLICT (slug) DO UPDATE SET
  name_fr                   = EXCLUDED.name_fr,
  name_en                   = EXCLUDED.name_en,
  price_monthly             = EXCLUDED.price_monthly,
  price_annual              = EXCLUDED.price_annual,
  tier                      = EXCLUDED.tier,
  device_range_fr           = EXCLUDED.device_range_fr,
  device_range_en           = EXCLUDED.device_range_en,
  features_fr               = EXCLUDED.features_fr,
  features_en               = EXCLUDED.features_en,
  not_covered_fr            = EXCLUDED.not_covered_fr,
  not_covered_en            = EXCLUDED.not_covered_en,
  service_time              = EXCLUDED.service_time,
  is_popular                = EXCLUDED.is_popular,
  sort_order                = EXCLUDED.sort_order,
  market                    = EXCLUDED.market,
  currency                  = EXCLUDED.currency,
  max_smartphones           = EXCLUDED.max_smartphones,
  max_tablets               = EXCLUDED.max_tablets,
  max_computers             = EXCLUDED.max_computers,
  max_game_consoles         = EXCLUDED.max_game_consoles,
  max_tvs                   = EXCLUDED.max_tvs,
  claim_waiting_period_days = EXCLUDED.claim_waiting_period_days,
  updated_at                = now();

-- Dev test plan keeps waiting_period = 0 so internal QA can claim immediately.
UPDATE plans
SET claim_waiting_period_days = 0
WHERE slug = 'test-plan-dev';
