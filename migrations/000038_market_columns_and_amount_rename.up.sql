-- Add market column to entities that need per-row currency context. The
-- column is denormalised from users.market for fast filtered admin queries
-- without per-row JOINs. Default 'SN' is safe — all existing rows are SN.
ALTER TABLE users           ADD COLUMN IF NOT EXISTS market VARCHAR(2) NOT NULL DEFAULT 'SN';
ALTER TABLE devices         ADD COLUMN IF NOT EXISTS market VARCHAR(2) NOT NULL DEFAULT 'SN';
ALTER TABLE repair_bookings ADD COLUMN IF NOT EXISTS market VARCHAR(2) NOT NULL DEFAULT 'SN';
ALTER TABLE claims          ADD COLUMN IF NOT EXISTS market VARCHAR(2) NOT NULL DEFAULT 'SN';
ALTER TABLE payments        ADD COLUMN IF NOT EXISTS market VARCHAR(2) NOT NULL DEFAULT 'SN';

-- Rename amount columns to currency-agnostic names. Stored in minor units:
--   XOF: whole units (3500 stored, displayed as 3 500 FCFA)
--   USD: cents          (1499 stored, displayed as $14.99)
-- Currency is derived from the row's market at display time — never stored
-- separately to avoid market='US'/currency='XOF' desync.
ALTER TABLE repair_bookings RENAME COLUMN repair_amount_xof TO repair_amount_minor;
ALTER TABLE claims          RENAME COLUMN amount_xof TO amount_minor;
ALTER TABLE payments        RENAME COLUMN amount_xof TO amount_minor;

-- Indexes optimised for admin "list by market" + status filters.
CREATE INDEX IF NOT EXISTS users_market_idx                   ON users(market);
CREATE INDEX IF NOT EXISTS devices_market_status_idx          ON devices(market, status);
CREATE INDEX IF NOT EXISTS repair_bookings_market_status_idx  ON repair_bookings(market, status);
CREATE INDEX IF NOT EXISTS claims_market_status_idx           ON claims(market, status);
CREATE INDEX IF NOT EXISTS payments_market_idx                ON payments(market);
