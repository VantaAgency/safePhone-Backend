UPDATE plans
SET
  device_range_fr = 'Smartphones jusqu''à 100 000 XOF',
  device_range_en = 'Smartphones up to 100,000 XOF',
  features_fr = '["Casse écran (avant)", "Assistance déclaration", "Suivi en ligne"]'::jsonb,
  features_en = '["Front screen damage", "Claim assistance", "Online tracking"]'::jsonb,
  not_covered_fr = '["Vol", "Oxydation", "Dommages esthétiques"]'::jsonb,
  not_covered_en = '["Theft", "Water damage", "Cosmetic damage"]'::jsonb
WHERE slug = 'essentiel';

UPDATE plans
SET
  device_range_fr = 'Smartphones jusqu''à 250 000 XOF',
  device_range_en = 'Smartphones up to 250,000 XOF',
  features_fr = '["Casse écran (avant + arrière)", "Assistance déclaration", "Suivi en ligne", "Prise en charge rapide"]'::jsonb,
  features_en = '["Screen damage (front + back)", "Claim assistance", "Online tracking", "Fast processing"]'::jsonb,
  not_covered_fr = '["Vol", "Oxydation", "Dommages esthétiques"]'::jsonb,
  not_covered_en = '["Theft", "Water damage", "Cosmetic damage"]'::jsonb
WHERE slug = 'ecran-plus';

UPDATE plans
SET
  device_range_fr = 'Smartphones jusqu''à 400 000 XOF',
  device_range_en = 'Smartphones up to 400,000 XOF',
  features_fr = '["Casse écran", "Panne & défaillance", "Oxydation", "Suivi en temps réel", "Réparation sous 48h"]'::jsonb,
  features_en = '["Screen damage", "Breakdown & malfunction", "Water damage", "Real-time tracking", "Repair within 48h"]'::jsonb,
  not_covered_fr = '["Vol", "Dommages esthétiques"]'::jsonb,
  not_covered_en = '["Theft", "Cosmetic damage"]'::jsonb
WHERE slug = 'plus';

UPDATE plans
SET
  device_range_fr = 'Smartphones jusqu''à 700 000 XOF',
  device_range_en = 'Smartphones up to 700,000 XOF',
  features_fr = '["Casse écran", "Panne & défaillance", "Oxydation", "Vol & perte", "Suivi en temps réel", "Réparation sous 24h", "Appareil de prêt"]'::jsonb,
  features_en = '["Screen damage", "Breakdown & malfunction", "Water damage", "Theft & loss", "Real-time tracking", "Repair within 24h", "Loaner device"]'::jsonb,
  not_covered_fr = '["Dommages esthétiques"]'::jsonb,
  not_covered_en = '["Cosmetic damage"]'::jsonb
WHERE slug = 'haute';

UPDATE plans
SET
  device_range_fr = 'Tous smartphones',
  device_range_en = 'All smartphones',
  features_fr = '["Casse écran", "Panne & défaillance", "Oxydation", "Vol & perte", "Multi-appareils (jusqu''à 3)", "Suivi en temps réel", "Réparation sous 12h", "Appareil de prêt", "Support prioritaire"]'::jsonb,
  features_en = '["Screen damage", "Breakdown & malfunction", "Water damage", "Theft & loss", "Multi-device (up to 3)", "Real-time tracking", "Repair within 12h", "Loaner device", "Priority support"]'::jsonb,
  not_covered_fr = '[]'::jsonb,
  not_covered_en = '[]'::jsonb
WHERE slug = 'totale';
