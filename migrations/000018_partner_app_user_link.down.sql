DROP INDEX IF EXISTS idx_partner_applications_unique_pending;
DROP INDEX IF EXISTS idx_partner_applications_org;
DROP INDEX IF EXISTS idx_partner_applications_user;

ALTER TABLE partner_applications
  DROP COLUMN IF EXISTS rejection_reason,
  DROP COLUMN IF EXISTS reviewed_by,
  DROP COLUMN IF EXISTS org_id,
  DROP COLUMN IF EXISTS user_id;
