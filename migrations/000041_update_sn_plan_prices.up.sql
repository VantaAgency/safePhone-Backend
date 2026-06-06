-- Update SN plan prices to the published FCFA rate card (matches the public
-- /sn/forfaits page). price_annual keeps the existing 10x (two-months-free)
-- ratio relative to price_monthly. ONLY prices change — names, coverage,
-- features and device matrix are untouched.
UPDATE plans SET price_monthly = 1500,  price_annual = 15000  WHERE market = 'SN' AND slug = 'essentiel';
UPDATE plans SET price_monthly = 3500,  price_annual = 35000  WHERE market = 'SN' AND slug = 'ecran-plus';
UPDATE plans SET price_monthly = 6000,  price_annual = 60000  WHERE market = 'SN' AND slug = 'plus';
UPDATE plans SET price_monthly = 10000, price_annual = 100000 WHERE market = 'SN' AND slug = 'haute';
UPDATE plans SET price_monthly = 15000, price_annual = 150000 WHERE market = 'SN' AND slug = 'totale';
