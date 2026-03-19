-- Ensure the canonical shared org exists for public repair requests.
INSERT INTO organizations (id, name, slug, plan, created_at, updated_at)
VALUES (gen_random_uuid(), 'SafePhone', 'safephone', 'free', NOW(), NOW())
ON CONFLICT (slug) DO NOTHING;

ALTER TABLE repair_bookings
    RENAME COLUMN device_type TO device_brand;

ALTER TABLE repair_bookings
    RENAME COLUMN booking_date TO preferred_date;

ALTER TABLE repair_bookings
    RENAME COLUMN booking_time TO preferred_time;

ALTER TABLE repair_bookings
    ADD COLUMN device_model VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN service_mode VARCHAR(20),
    ADD COLUMN center_id VARCHAR(100),
    ADD COLUMN scheduled_date DATE,
    ADD COLUMN scheduled_time VARCHAR(10),
    ADD COLUMN repair_amount_xof INTEGER,
    ADD COLUMN request_source VARCHAR(20) NOT NULL DEFAULT 'public_visitor',
    ADD COLUMN customer_phone_normalized VARCHAR(30) NOT NULL DEFAULT '';

UPDATE repair_bookings
SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id IS NULL;

UPDATE repair_bookings
SET status = 'pending'
WHERE status = 'confirmed';

UPDATE repair_bookings
SET
    service_mode = CASE
        WHEN location_id = 'home' THEN 'home'
        ELSE 'center'
    END,
    center_id = CASE
        WHEN location_id = 'home' THEN NULL
        ELSE location_id
    END,
    customer_phone_normalized = regexp_replace(customer_phone, '\D', '', 'g');

ALTER TABLE repair_bookings
    DROP COLUMN location_id;

ALTER TABLE repair_bookings
    ALTER COLUMN org_id SET NOT NULL,
    ALTER COLUMN service_mode SET NOT NULL,
    ALTER COLUMN status SET DEFAULT 'pending',
    ALTER COLUMN device_model DROP DEFAULT,
    ALTER COLUMN customer_phone_normalized DROP DEFAULT;

ALTER TABLE repair_bookings
    ADD CONSTRAINT chk_repair_bookings_service_mode
        CHECK (service_mode IN ('center', 'home')),
    ADD CONSTRAINT chk_repair_bookings_status
        CHECK (status IN ('pending', 'accepted', 'rejected', 'scheduled', 'in_progress', 'completed', 'cancelled')),
    ADD CONSTRAINT chk_repair_bookings_request_source
        CHECK (request_source IN ('public_visitor', 'safephone_user')),
    ADD CONSTRAINT chk_repair_bookings_amount_nonnegative
        CHECK (repair_amount_xof IS NULL OR repair_amount_xof >= 0);

CREATE INDEX IF NOT EXISTS idx_repair_bookings_org_created_at
    ON repair_bookings(org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_repair_bookings_user_created_at
    ON repair_bookings(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_repair_bookings_reference_phone_lookup
    ON repair_bookings(reference, customer_phone_normalized);
