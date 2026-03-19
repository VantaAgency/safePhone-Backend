ALTER TABLE plans
  ADD COLUMN dexpay_product_id_monthly VARCHAR(100),
  ADD COLUMN dexpay_product_id_annual  VARCHAR(100);

ALTER TABLE users
  ADD COLUMN dexpay_customer_id VARCHAR(100);

ALTER TABLE subscriptions
  ADD COLUMN dexpay_subscription_id     VARCHAR(100),
  ADD COLUMN dexpay_checkout_session_id VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_subscriptions_dexpay ON subscriptions(dexpay_subscription_id);
CREATE INDEX IF NOT EXISTS idx_users_dexpay ON users(dexpay_customer_id);

DROP INDEX IF EXISTS idx_payments_plan_id;
DROP INDEX IF EXISTS idx_payments_provider_ref;

ALTER TABLE payments
  DROP COLUMN IF EXISTS provider_payload,
  DROP COLUMN IF EXISTS failed_at,
  DROP COLUMN IF EXISTS payment_url,
  DROP COLUMN IF EXISTS currency,
  DROP COLUMN IF EXISTS provider,
  DROP COLUMN IF EXISTS plan_id;
