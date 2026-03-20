ALTER TABLE partner_applications
  ADD COLUMN IF NOT EXISTS business_location VARCHAR(200);

UPDATE partner_applications
SET business_location = city
WHERE business_location IS NULL OR btrim(business_location) = '';

ALTER TABLE partner_applications
  ALTER COLUMN business_location SET NOT NULL;

ALTER TABLE partners
  RENAME COLUMN commission_rate TO commission_percentage;

ALTER TABLE partners
  ALTER COLUMN commission_percentage TYPE NUMERIC(5,2)
  USING CASE
    WHEN commission_percentage <= 1 THEN round((commission_percentage * 100)::numeric, 2)
    ELSE round(commission_percentage::numeric, 2)
  END;

ALTER TABLE partners
  ALTER COLUMN commission_percentage DROP DEFAULT;

ALTER TABLE partners
  ADD COLUMN IF NOT EXISTS business_location VARCHAR(200);

UPDATE partners
SET business_location = city
WHERE business_location IS NULL OR btrim(business_location) = '';

ALTER TABLE partners
  ALTER COLUMN business_location SET NOT NULL;

WITH ranked_clients AS (
  SELECT
    id,
    row_number() OVER (
      PARTITION BY linked_user_id
      ORDER BY
        COALESCE(invitation_claimed_at, invited_at, created_at) ASC,
        created_at ASC,
        id ASC
    ) AS row_num
  FROM partner_clients
  WHERE linked_user_id IS NOT NULL
)
UPDATE partner_clients pc
SET linked_user_id = NULL,
    invitation_claimed_at = NULL,
    updated_at = now()
FROM ranked_clients rc
WHERE pc.id = rc.id
  AND rc.row_num > 1;

DROP INDEX IF EXISTS idx_partner_clients_linked_user_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_partner_clients_linked_user_id_unique
  ON partner_clients(linked_user_id)
  WHERE linked_user_id IS NOT NULL;

ALTER TABLE partner_commissions
  RENAME COLUMN amount_xof TO commission_amount_xof;

ALTER TABLE partner_commissions
  ADD COLUMN IF NOT EXISTS partner_client_id UUID REFERENCES partner_clients(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS client_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS plan_id UUID REFERENCES plans(id),
  ADD COLUMN IF NOT EXISTS base_amount_xof INTEGER,
  ADD COLUMN IF NOT EXISTS commission_percentage NUMERIC(5,2),
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE partner_commissions comm
SET
  client_user_id = pay.user_id,
  partner_client_id = (
    SELECT pc.id
    FROM partner_clients pc
    WHERE pc.partner_id = comm.partner_id
      AND pc.linked_user_id = pay.user_id
    ORDER BY
      COALESCE(pc.invitation_claimed_at, pc.invited_at, pc.created_at) ASC,
      pc.created_at ASC,
      pc.id ASC
    LIMIT 1
  ),
  plan_id = pay.plan_id,
  base_amount_xof = pay.amount_xof,
  commission_percentage = (
    SELECT p.commission_percentage
    FROM partners p
    WHERE p.id = comm.partner_id
  ),
  updated_at = now()
FROM payments pay
WHERE comm.payment_id = pay.id;

CREATE INDEX IF NOT EXISTS idx_partner_commissions_partner_client_id
  ON partner_commissions(partner_client_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_partner_commissions_payment_id_unique
  ON partner_commissions(payment_id)
  WHERE payment_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_partner_commissions_client_user_id_unique
  ON partner_commissions(client_user_id)
  WHERE client_user_id IS NOT NULL;
