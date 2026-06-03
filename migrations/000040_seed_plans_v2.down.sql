-- Rolling back the seed is non-destructive: we restore the previous v1
-- content (pre-000040). This recovers the legacy features/not_covered
-- copy from 000008 (SN) and 000036 (US). max_* columns reset to 0 so
-- a fresh roll-forward of 000040 picks up the right counts.

UPDATE plans
SET max_smartphones        = 0,
    max_tablets            = 0,
    max_computers          = 0,
    max_game_consoles      = 0,
    max_tvs                = 0,
    claim_waiting_period_days = 30,
    updated_at             = now()
WHERE slug IN (
  'essentiel', 'ecran-plus', 'plus', 'haute', 'totale',
  'us_essentiel', 'us_ecran_plus', 'us_plus', 'us_premium', 'us_total'
);

-- us_essentiel goes back to $9.99
UPDATE plans
SET price_monthly = 999,
    price_annual  = 9990,
    updated_at    = now()
WHERE slug = 'us_essentiel';
