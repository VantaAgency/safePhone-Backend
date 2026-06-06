-- Restore the previous SN plan prices.
UPDATE plans SET price_monthly = 15000,  price_annual = 150000  WHERE market = 'SN' AND slug = 'essentiel';
UPDATE plans SET price_monthly = 35000,  price_annual = 350000  WHERE market = 'SN' AND slug = 'ecran-plus';
UPDATE plans SET price_monthly = 60000,  price_annual = 600000  WHERE market = 'SN' AND slug = 'plus';
UPDATE plans SET price_monthly = 100000, price_annual = 1000000 WHERE market = 'SN' AND slug = 'haute';
UPDATE plans SET price_monthly = 150000, price_annual = 1500000 WHERE market = 'SN' AND slug = 'totale';
