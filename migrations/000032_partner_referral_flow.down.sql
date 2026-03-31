DROP INDEX IF EXISTS idx_partner_clients_partner_attributed_at;
DROP INDEX IF EXISTS idx_partner_clients_referral_code;

ALTER TABLE partner_clients
  DROP COLUMN IF EXISTS attributed_at,
  DROP COLUMN IF EXISTS referral_medium,
  DROP COLUMN IF EXISTS referral_code,
  DROP COLUMN IF EXISTS attribution_source;

DROP INDEX IF EXISTS idx_partner_referral_visits_partner_source;
DROP INDEX IF EXISTS idx_partner_referral_visits_partner_visited_at;
DROP TABLE IF EXISTS partner_referral_visits;

DROP INDEX IF EXISTS idx_partners_referral_code_unique;

ALTER TABLE partners
  DROP COLUMN IF EXISTS referral_code;
