DROP INDEX IF EXISTS idx_partner_commissions_client_user_id_unique;
DROP INDEX IF EXISTS idx_partner_commissions_payment_id_unique;
DROP INDEX IF EXISTS idx_partner_commissions_partner_client_id;

ALTER TABLE partner_commissions
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS commission_percentage,
  DROP COLUMN IF EXISTS base_amount_xof,
  DROP COLUMN IF EXISTS plan_id,
  DROP COLUMN IF EXISTS client_user_id,
  DROP COLUMN IF EXISTS partner_client_id;

ALTER TABLE partner_commissions
  RENAME COLUMN commission_amount_xof TO amount_xof;

DROP INDEX IF EXISTS idx_partner_clients_linked_user_id_unique;

CREATE INDEX IF NOT EXISTS idx_partner_clients_linked_user_id
  ON partner_clients(linked_user_id);

ALTER TABLE partners
  DROP COLUMN IF EXISTS business_location;

ALTER TABLE partners
  RENAME COLUMN commission_percentage TO commission_rate;

ALTER TABLE partners
  ALTER COLUMN commission_rate SET DEFAULT 20.00;

ALTER TABLE partner_applications
  DROP COLUMN IF EXISTS business_location;
