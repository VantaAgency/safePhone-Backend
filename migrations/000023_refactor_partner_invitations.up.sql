ALTER TABLE partner_clients
  ADD COLUMN linked_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN invitation_token VARCHAR(120),
  ADD COLUMN invitation_expires_at TIMESTAMPTZ,
  ADD COLUMN invitation_claimed_at TIMESTAMPTZ;

UPDATE partner_clients
SET invitation_token = gen_random_uuid()::text
WHERE invitation_token IS NULL;

UPDATE partner_clients
SET invitation_expires_at = invited_at + INTERVAL '30 days'
WHERE invitation_expires_at IS NULL;

UPDATE partner_clients
SET status = CASE
  WHEN status = 'active' THEN 'active'
  WHEN status IN ('plan_purchased', 'device_registered') THEN 'payment_pending'
  ELSE 'invited'
END;

ALTER TABLE partner_clients
  ALTER COLUMN invitation_token SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_partner_clients_invitation_token ON partner_clients(invitation_token);
CREATE INDEX IF NOT EXISTS idx_partner_clients_linked_user_id ON partner_clients(linked_user_id);
