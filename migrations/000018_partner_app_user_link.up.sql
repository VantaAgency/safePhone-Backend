-- Remove anonymous submissions that pre-date the user-linked model
TRUNCATE TABLE partner_applications;

ALTER TABLE partner_applications
  ADD COLUMN user_id UUID NOT NULL REFERENCES users(id),
  ADD COLUMN org_id  UUID NOT NULL REFERENCES organizations(id),
  ADD COLUMN reviewed_by UUID REFERENCES users(id),
  ADD COLUMN rejection_reason TEXT;

CREATE INDEX idx_partner_applications_user ON partner_applications(user_id);
CREATE INDEX idx_partner_applications_org  ON partner_applications(org_id);
CREATE UNIQUE INDEX idx_partner_applications_unique_pending
  ON partner_applications(org_id, user_id) WHERE status = 'pending';
