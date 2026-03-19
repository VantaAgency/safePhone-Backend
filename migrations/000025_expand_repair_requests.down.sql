DROP INDEX IF EXISTS idx_repair_bookings_reference_phone_lookup;
DROP INDEX IF EXISTS idx_repair_bookings_user_created_at;
DROP INDEX IF EXISTS idx_repair_bookings_org_created_at;

ALTER TABLE repair_bookings
    DROP CONSTRAINT IF EXISTS chk_repair_bookings_service_mode,
    DROP CONSTRAINT IF EXISTS chk_repair_bookings_status,
    DROP CONSTRAINT IF EXISTS chk_repair_bookings_request_source,
    DROP CONSTRAINT IF EXISTS chk_repair_bookings_amount_nonnegative;

ALTER TABLE repair_bookings
    ADD COLUMN location_id VARCHAR(100);

UPDATE repair_bookings
SET location_id = CASE
    WHEN service_mode = 'home' THEN 'home'
    ELSE COALESCE(center_id, 'home')
END;

UPDATE repair_bookings
SET status = CASE
    WHEN status IN ('pending', 'accepted', 'scheduled', 'in_progress') THEN 'confirmed'
    ELSE status
END;

ALTER TABLE repair_bookings
    ALTER COLUMN location_id SET NOT NULL,
    ALTER COLUMN org_id DROP NOT NULL,
    ALTER COLUMN status SET DEFAULT 'confirmed';

ALTER TABLE repair_bookings
    DROP COLUMN customer_phone_normalized,
    DROP COLUMN request_source,
    DROP COLUMN repair_amount_xof,
    DROP COLUMN scheduled_time,
    DROP COLUMN scheduled_date,
    DROP COLUMN center_id,
    DROP COLUMN service_mode,
    DROP COLUMN device_model;

ALTER TABLE repair_bookings
    RENAME COLUMN device_brand TO device_type;

ALTER TABLE repair_bookings
    RENAME COLUMN preferred_date TO booking_date;

ALTER TABLE repair_bookings
    RENAME COLUMN preferred_time TO booking_time;
