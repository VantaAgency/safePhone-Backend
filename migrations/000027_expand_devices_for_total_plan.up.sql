ALTER TABLE devices
  ADD COLUMN IF NOT EXISTS device_type VARCHAR(32) NOT NULL DEFAULT 'smartphone',
  ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE devices
SET
  device_type = COALESCE(NULLIF(btrim(device_type), ''), 'smartphone'),
  metadata = COALESCE(metadata, '{}'::jsonb);

ALTER TABLE devices
  ADD CONSTRAINT devices_device_type_check
  CHECK (device_type IN ('smartphone', 'tablet', 'tv', 'computer', 'home_electronics'));
