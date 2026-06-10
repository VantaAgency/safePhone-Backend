-- Revert to the (inflated) 000040_seed_plans_v2 SN prices.
UPDATE plans AS p SET
  price_monthly = v.monthly,
  price_annual  = v.annual,
  updated_at    = now()
FROM (VALUES
  ('essentiel',   15000,  150000),
  ('ecran-plus',  35000,  350000),
  ('plus',        60000,  600000),
  ('haute',      100000, 1000000),
  ('totale',     150000, 1500000)
) AS v(slug, monthly, annual)
WHERE p.slug = v.slug AND p.market = 'SN';
