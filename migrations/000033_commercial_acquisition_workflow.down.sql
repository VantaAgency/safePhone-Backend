DROP TABLE IF EXISTS commercial_activity_reports;
DROP TABLE IF EXISTS commercial_commissions;

ALTER TABLE partners
  DROP COLUMN IF EXISTS acquisition_source,
  DROP COLUMN IF EXISTS commercial_id;

ALTER TABLE partner_applications
  DROP COLUMN IF EXISTS acquisition_source,
  DROP COLUMN IF EXISTS commercial_id;

DROP TABLE IF EXISTS commercial_profiles;
