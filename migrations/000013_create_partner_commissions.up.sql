CREATE TABLE IF NOT EXISTS partner_commissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    partner_id UUID NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
    payment_id UUID REFERENCES payments(id),
    amount_xof INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    payout_method VARCHAR(50),
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_partner_commissions_partner_id ON partner_commissions(partner_id);
CREATE INDEX IF NOT EXISTS idx_partner_commissions_org_id ON partner_commissions(org_id);
