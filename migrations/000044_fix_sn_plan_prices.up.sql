-- Fix SN plan prices: 000040_seed_plans_v2 inflated every SN (XOF) price by
-- 10× (e.g. Essentiel 1 500 -> 15 000 FCFA/mo). XOF renders with no minor-unit
-- division on the frontend, so the DB value is the displayed price — and
-- PaymentPage charges price_monthly directly via DEXPAY, so the inflation also
-- over-billed customers. Restore the published flyer prices (== seed 000008).
-- US (USD cents) prices are correct and intentionally left untouched.

UPDATE plans AS p SET
  price_monthly = v.monthly,
  price_annual  = v.annual,
  updated_at    = now()
FROM (VALUES
  ('essentiel',   1500,  15000),
  ('ecran-plus',  3500,  35000),
  ('plus',        6000,  60000),
  ('haute',      10000, 100000),
  ('totale',     15000, 150000)
) AS v(slug, monthly, annual)
WHERE p.slug = v.slug AND p.market = 'SN';
