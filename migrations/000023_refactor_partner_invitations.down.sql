DROP INDEX IF EXISTS idx_partner_clients_linked_user_id;
DROP INDEX IF EXISTS idx_partner_clients_invitation_token;

ALTER TABLE partner_clients
  DROP COLUMN IF EXISTS invitation_claimed_at,
  DROP COLUMN IF EXISTS invitation_expires_at,
  DROP COLUMN IF EXISTS invitation_token,
  DROP COLUMN IF EXISTS linked_user_id;
