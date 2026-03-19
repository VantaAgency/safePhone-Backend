CREATE TABLE IF NOT EXISTS partner_clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    partner_id UUID NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
    client_name VARCHAR(200) NOT NULL,
    client_phone VARCHAR(30),
    plan_id UUID REFERENCES plans(id),
    status VARCHAR(30) NOT NULL DEFAULT 'invited',
    invited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_partner_clients_partner_id ON partner_clients(partner_id);
CREATE INDEX IF NOT EXISTS idx_partner_clients_org_id ON partner_clients(org_id);
