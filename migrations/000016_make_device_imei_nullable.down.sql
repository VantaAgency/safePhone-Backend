DROP INDEX IF EXISTS idx_devices_imei;
UPDATE devices SET imei = '000000000000000' WHERE imei IS NULL;
ALTER TABLE devices ALTER COLUMN imei SET NOT NULL;
CREATE UNIQUE INDEX idx_devices_imei ON devices(imei);
