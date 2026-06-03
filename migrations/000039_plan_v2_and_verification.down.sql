-- Reverse of 000039: drop everything added.

DROP INDEX IF EXISTS devices_pending_verification_idx;
ALTER TABLE devices
  DROP COLUMN IF EXISTS verification_photos,
  DROP COLUMN IF EXISTS verification_video,
  DROP COLUMN IF EXISTS verification_status,
  DROP COLUMN IF EXISTS verified_at,
  DROP COLUMN IF EXISTS verified_by,
  DROP COLUMN IF EXISTS verification_rejected_reason;

DROP INDEX IF EXISTS subscription_devices_device_id_idx;
DROP TABLE IF EXISTS subscription_devices;

ALTER TABLE subscriptions
  DROP COLUMN IF EXISTS activated_at;

ALTER TABLE plans
  DROP COLUMN IF EXISTS max_smartphones,
  DROP COLUMN IF EXISTS max_tablets,
  DROP COLUMN IF EXISTS max_computers,
  DROP COLUMN IF EXISTS max_game_consoles,
  DROP COLUMN IF EXISTS max_tvs,
  DROP COLUMN IF EXISTS claim_waiting_period_days;
