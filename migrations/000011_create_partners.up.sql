CREATE TABLE IF NOT EXISTS partners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    store_name VARCHAR(200) NOT NULL,
    city VARCHAR(100) NOT NULL,
    commission_rate NUMERIC(5,2) NOT NULL DEFAULT 20.00,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_partners_org_id ON partners(org_id);
CREATE INDEX IF NOT EXISTS idx_partners_user_id ON partners(user_id);
