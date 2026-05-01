CREATE TABLE IF NOT EXISTS commercial_profiles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  referral_code VARCHAR(16) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  commission_percentage NUMERIC(5,2) NOT NULL DEFAULT 5.00,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(org_id, user_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_commercial_profiles_referral_code_unique
  ON commercial_profiles(referral_code);

CREATE INDEX IF NOT EXISTS idx_commercial_profiles_org_status
  ON commercial_profiles(org_id, status);

ALTER TABLE partner_applications
  ADD COLUMN IF NOT EXISTS commercial_id UUID REFERENCES commercial_profiles(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS acquisition_source VARCHAR(40) NOT NULL DEFAULT 'direct';

CREATE INDEX IF NOT EXISTS idx_partner_applications_commercial_id
  ON partner_applications(commercial_id);

ALTER TABLE partners
  ADD COLUMN IF NOT EXISTS commercial_id UUID REFERENCES commercial_profiles(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS acquisition_source VARCHAR(40) NOT NULL DEFAULT 'direct';

CREATE INDEX IF NOT EXISTS idx_partners_commercial_id
  ON partners(commercial_id);

CREATE TABLE IF NOT EXISTS commercial_commissions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  commercial_id UUID NOT NULL REFERENCES commercial_profiles(id) ON DELETE CASCADE,
  partner_id UUID NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  partner_client_id UUID REFERENCES partner_clients(id) ON DELETE SET NULL,
  client_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  payment_id UUID REFERENCES payments(id) ON DELETE SET NULL,
  plan_id UUID REFERENCES plans(id) ON DELETE SET NULL,
  base_amount_xof INTEGER NOT NULL,
  commission_percentage NUMERIC(5,2) NOT NULL,
  commission_amount_xof INTEGER NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  paid_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_commercial_commissions_partner_unique
  ON commercial_commissions(partner_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_commercial_commissions_payment_unique
  ON commercial_commissions(payment_id)
  WHERE payment_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_commercial_commissions_commercial_created
  ON commercial_commissions(commercial_id, created_at DESC);

CREATE TABLE IF NOT EXISTS commercial_activity_reports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  commercial_id UUID NOT NULL REFERENCES commercial_profiles(id) ON DELETE CASCADE,
  partner_id UUID REFERENCES partners(id) ON DELETE SET NULL,
  prospect_name VARCHAR(200),
  activity_type VARCHAR(40) NOT NULL,
  comment TEXT NOT NULL,
  city VARCHAR(100),
  location VARCHAR(200),
  photo_url TEXT NOT NULL,
  photo_storage_path TEXT NOT NULL,
  photo_content_type VARCHAR(100) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_commercial_activity_reports_commercial_created
  ON commercial_activity_reports(commercial_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_commercial_activity_reports_partner_created
  ON commercial_activity_reports(partner_id, created_at DESC)
  WHERE partner_id IS NOT NULL;
