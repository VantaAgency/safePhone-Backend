ALTER TABLE partners
  ADD COLUMN IF NOT EXISTS referral_code VARCHAR(16);

UPDATE partners
SET referral_code = upper(substr(replace(id::text, '-', ''), 1, 8))
WHERE referral_code IS NULL OR btrim(referral_code) = '';

ALTER TABLE partners
  ALTER COLUMN referral_code SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_partners_referral_code_unique
  ON partners(referral_code);

CREATE TABLE IF NOT EXISTS partner_referral_visits (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  partner_id UUID NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  referral_code VARCHAR(16) NOT NULL,
  visitor_token VARCHAR(120) NOT NULL,
  source_medium VARCHAR(20) NOT NULL DEFAULT 'unknown',
  visited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_partner_referral_visits_partner_visited_at
  ON partner_referral_visits(partner_id, visited_at DESC);

CREATE INDEX IF NOT EXISTS idx_partner_referral_visits_partner_source
  ON partner_referral_visits(partner_id, source_medium, visited_at DESC);

ALTER TABLE partner_clients
  ADD COLUMN IF NOT EXISTS attribution_source VARCHAR(40) NOT NULL DEFAULT 'manual_invitation',
  ADD COLUMN IF NOT EXISTS referral_code VARCHAR(16),
  ADD COLUMN IF NOT EXISTS referral_medium VARCHAR(20) NOT NULL DEFAULT 'unknown',
  ADD COLUMN IF NOT EXISTS attributed_at TIMESTAMPTZ;

UPDATE partner_clients pc
SET
  attribution_source = COALESCE(NULLIF(pc.attribution_source, ''), 'manual_invitation'),
  referral_code = COALESCE(pc.referral_code, p.referral_code),
  referral_medium = COALESCE(NULLIF(pc.referral_medium, ''), 'unknown'),
  attributed_at = COALESCE(pc.attributed_at, pc.invitation_claimed_at, pc.invited_at, pc.created_at)
FROM partners p
WHERE p.id = pc.partner_id;

CREATE INDEX IF NOT EXISTS idx_partner_clients_referral_code
  ON partner_clients(referral_code);

CREATE INDEX IF NOT EXISTS idx_partner_clients_partner_attributed_at
  ON partner_clients(partner_id, attributed_at DESC);
