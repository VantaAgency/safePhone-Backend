DELETE FROM plans WHERE slug = 'us_plus_monthly';

UPDATE plans SET price_annual = 0 WHERE slug = 'us_essential_monthly';
UPDATE plans SET price_annual = 0 WHERE slug = 'us_complete_monthly';
