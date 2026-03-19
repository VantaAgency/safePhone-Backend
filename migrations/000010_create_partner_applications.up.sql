CREATE TABLE IF NOT EXISTS partner_applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_name VARCHAR(200) NOT NULL,
    full_name VARCHAR(200) NOT NULL,
    phone VARCHAR(30) NOT NULL,
    city VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_partner_applications_status ON partner_applications(status);
