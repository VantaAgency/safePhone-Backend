DROP INDEX IF EXISTS payments_market_idx;
DROP INDEX IF EXISTS claims_market_status_idx;
DROP INDEX IF EXISTS repair_bookings_market_status_idx;
DROP INDEX IF EXISTS devices_market_status_idx;
DROP INDEX IF EXISTS users_market_idx;

ALTER TABLE payments        RENAME COLUMN amount_minor TO amount_xof;
ALTER TABLE claims          RENAME COLUMN amount_minor TO amount_xof;
ALTER TABLE repair_bookings RENAME COLUMN repair_amount_minor TO repair_amount_xof;

ALTER TABLE payments        DROP COLUMN IF EXISTS market;
ALTER TABLE claims          DROP COLUMN IF EXISTS market;
ALTER TABLE repair_bookings DROP COLUMN IF EXISTS market;
ALTER TABLE devices         DROP COLUMN IF EXISTS market;
ALTER TABLE users           DROP COLUMN IF EXISTS market;
