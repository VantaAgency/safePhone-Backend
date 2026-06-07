-- game_console was added as a device type across the app (domain enum, request
-- validators, frontend pickers, plan coverage caps) but the devices CHECK
-- constraint was never updated, so inserting a console device fails with
-- devices_device_type_check (SQLSTATE 23514) — both when paying for one and
-- when adding one to a subscription. Expand the allowed set. home_electronics
-- is kept so any legacy rows still validate.
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_device_type_check;
ALTER TABLE devices ADD CONSTRAINT devices_device_type_check
  CHECK (device_type IN (
    'smartphone', 'tablet', 'tv', 'computer', 'game_console', 'home_electronics'
  ));
