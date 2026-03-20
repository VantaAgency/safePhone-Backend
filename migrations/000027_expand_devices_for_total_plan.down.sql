ALTER TABLE devices
  DROP CONSTRAINT IF EXISTS devices_device_type_check;

ALTER TABLE devices
  DROP COLUMN IF EXISTS metadata,
  DROP COLUMN IF EXISTS device_type;
