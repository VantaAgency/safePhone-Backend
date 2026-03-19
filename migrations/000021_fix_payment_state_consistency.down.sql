DROP INDEX IF EXISTS idx_payments_subscription_created_at;

ALTER TABLE payments
  DROP COLUMN IF EXISTS expires_at;
