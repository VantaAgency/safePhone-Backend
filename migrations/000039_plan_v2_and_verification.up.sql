-- Plan v2 — multi-device coverage matrix + verification gate.
--
-- This migration adds the schema scaffolding for:
--   1. per-plan device-type coverage counts (max smartphones, tablets, …)
--   2. a 30-day claim waiting period per plan (configurable)
--   3. a new `pending_verification` subscription status + activated_at
--      timestamp set when an admin approves the photo/video proof
--   4. a subscription_devices join table so a single subscription can cover
--      multiple devices (legacy subscriptions.device_id stays in place as
--      the "primary device" until a follow-up migration removes it)
--   5. per-device verification fields (5 photos + 1 video + status + admin
--      reviewer)
--
-- Migration 000040 (next) seeds the new plan content into the existing rows.


-- 1. plans coverage columns + waiting period --------------------------------

ALTER TABLE plans
  ADD COLUMN IF NOT EXISTS max_smartphones        SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_tablets            SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_computers          SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_game_consoles      SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_tvs                SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS claim_waiting_period_days SMALLINT NOT NULL DEFAULT 30;


-- 2. subscriptions: activated_at + pending_verification status -------------
-- The status column is a VARCHAR enum string in Go; no Postgres ENUM type
-- to alter. The new value lands by data convention (service layer writes
-- 'pending_verification'). activated_at is null until an admin approves
-- the proof and the subscription transitions to 'active'.

ALTER TABLE subscriptions
  ADD COLUMN IF NOT EXISTS activated_at TIMESTAMPTZ;


-- 3. subscription_devices join table ---------------------------------------
-- Many-to-many between subscriptions and devices. Replaces the legacy
-- subscriptions.device_id single-FK relationship for plans with multi-device
-- coverage. The legacy column is kept for backwards compatibility this PR;
-- removal in a follow-up once all code paths read from this table.

CREATE TABLE IF NOT EXISTS subscription_devices (
  subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
  device_id       UUID NOT NULL REFERENCES devices(id)       ON DELETE RESTRICT,
  attached_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (subscription_id, device_id)
);

CREATE INDEX IF NOT EXISTS subscription_devices_device_id_idx
  ON subscription_devices(device_id);

-- Backfill from existing single-device subscriptions so the join table
-- mirrors the legacy state and nothing breaks on read.
INSERT INTO subscription_devices (subscription_id, device_id, attached_at)
SELECT id, device_id, COALESCE(created_at, now())
FROM subscriptions
WHERE device_id IS NOT NULL
ON CONFLICT (subscription_id, device_id) DO NOTHING;


-- 4. devices: verification fields ------------------------------------------
-- 5 photos required (TEXT[] of S3 URLs), 1 video required, admin review
-- pipeline mirrors the existing claims review flow.

ALTER TABLE devices
  ADD COLUMN IF NOT EXISTS verification_photos          TEXT[]       NOT NULL DEFAULT ARRAY[]::TEXT[],
  ADD COLUMN IF NOT EXISTS verification_video           TEXT,
  ADD COLUMN IF NOT EXISTS verification_status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS verified_at                  TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS verified_by                  UUID         REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS verification_rejected_reason TEXT;

-- Partial index for the admin Verifications queue — narrows the scan to
-- pending rows only since that's the hot query.
CREATE INDEX IF NOT EXISTS devices_pending_verification_idx
  ON devices(created_at DESC)
  WHERE verification_status = 'pending';

-- Pre-existing devices (created before this migration) shouldn't be locked
-- into the new verification workflow retroactively — mark them approved so
-- their claims keep flowing.
UPDATE devices
SET verification_status = 'approved',
    verified_at         = COALESCE(verified_at, created_at)
WHERE verification_status = 'pending'
  AND created_at < now();


-- 5. device_type enum extension: add 'game_console' ------------------------
-- DeviceType is a VARCHAR in Go, so the "enum extension" is purely a code
-- concern (NormalizeDeviceType picks it up). No DDL needed here, but we
-- ensure no existing row has the new value before code starts writing it
-- (sanity check kept as a comment for downstream reviewers):
--
--   SELECT COUNT(*) FROM devices WHERE device_type = 'game_console'; -- expect 0
