-- Revert to the original allowed set (without game_console). Fails if any
-- game_console rows exist — drop/convert them before rolling back.
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_device_type_check;
ALTER TABLE devices ADD CONSTRAINT devices_device_type_check
  CHECK (device_type IN (
    'smartphone', 'tablet', 'tv', 'computer', 'home_electronics'
  ));
