CREATE INDEX IF NOT EXISTS idx_devices_org_user_created_at
  ON devices(org_id, user_id, created_at DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_org_user_created_at
  ON subscriptions(org_id, user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_subscriptions_org_user_status
  ON subscriptions(org_id, user_id, status);

CREATE INDEX IF NOT EXISTS idx_claims_org_user_filed_at
  ON claims(org_id, user_id, filed_at DESC);

CREATE INDEX IF NOT EXISTS idx_payments_org_user_created_at
  ON payments(org_id, user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_payments_org_status_paid_at
  ON payments(org_id, status, paid_at DESC);

CREATE INDEX IF NOT EXISTS idx_partner_clients_partner_invited_at
  ON partner_clients(partner_id, invited_at DESC);
