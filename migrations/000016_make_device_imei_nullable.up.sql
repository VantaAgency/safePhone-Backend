-- IMEI is optional at registration time; users add it later from the dashboard.
-- NULL values are excluded from the unique index so multiple unregistered devices can coexist.
ALTER TABLE devices ALTER COLUMN imei DROP NOT NULL;
ALTER TABLE devices ALTER COLUMN imei TYPE VARCHAR(15);

-- Replace the unconditional unique constraint with a partial one that ignores NULLs.
DROP INDEX IF EXISTS idx_devices_imei;
CREATE UNIQUE INDEX idx_devices_imei ON devices(imei) WHERE imei IS NOT NULL AND deleted_at IS NULL;
