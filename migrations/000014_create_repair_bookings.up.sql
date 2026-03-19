CREATE TABLE IF NOT EXISTS repair_bookings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reference VARCHAR(20) NOT NULL UNIQUE,
    device_type VARCHAR(100) NOT NULL,
    repair_type VARCHAR(100) NOT NULL,
    location_id VARCHAR(100) NOT NULL,
    booking_date DATE NOT NULL,
    booking_time VARCHAR(10) NOT NULL,
    customer_name VARCHAR(200) NOT NULL,
    customer_phone VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'confirmed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_repair_bookings_reference ON repair_bookings(reference);
CREATE INDEX IF NOT EXISTS idx_repair_bookings_status ON repair_bookings(status);
