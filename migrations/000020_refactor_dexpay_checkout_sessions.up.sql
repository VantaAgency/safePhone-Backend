ALTER TABLE payments
  ADD COLUMN plan_id UUID REFERENCES plans(id) ON DELETE RESTRICT,
  ADD COLUMN provider VARCHAR(30) NOT NULL DEFAULT 'dexpay',
  ADD COLUMN currency VARCHAR(3) NOT NULL DEFAULT 'XOF',
  ADD COLUMN payment_url TEXT,
  ADD COLUMN failed_at TIMESTAMPTZ,
  ADD COLUMN provider_payload JSONB;

UPDATE payments pay
SET plan_id = sub.plan_id
FROM subscriptions sub
WHERE sub.id = pay.subscription_id
  AND pay.plan_id IS NULL;

ALTER TABLE payments
  ALTER COLUMN plan_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_ref ON payments(provider_ref) WHERE provider_ref IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_plan_id ON payments(plan_id);

DROP INDEX IF EXISTS idx_subscriptions_dexpay;
DROP INDEX IF EXISTS idx_users_dexpay;

ALTER TABLE subscriptions
  DROP COLUMN IF EXISTS dexpay_subscription_id,
  DROP COLUMN IF EXISTS dexpay_checkout_session_id;

ALTER TABLE users
  DROP COLUMN IF EXISTS dexpay_customer_id;

ALTER TABLE plans
  DROP COLUMN IF EXISTS dexpay_product_id_monthly,
  DROP COLUMN IF EXISTS dexpay_product_id_annual;
