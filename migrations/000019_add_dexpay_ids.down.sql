DROP TABLE IF EXISTS webhook_events;

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
