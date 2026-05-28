DROP INDEX IF EXISTS idx_plans_market;
DROP INDEX IF EXISTS idx_subscriptions_market_status;
DROP INDEX IF EXISTS idx_subscriptions_stripe_subscription;
DROP INDEX IF EXISTS idx_users_stripe_customer;

-- Restore device_id NOT NULL. Will fail if any US subscriptions exist
-- without a device — that's intentional; running this down migration
-- against mixed data isn't safe.
ALTER TABLE subscriptions
  ALTER COLUMN device_id SET NOT NULL;

ALTER TABLE plans
  DROP COLUMN IF EXISTS currency,
  DROP COLUMN IF EXISTS market;

ALTER TABLE subscriptions
  DROP COLUMN IF EXISTS stripe_checkout_session_id,
  DROP COLUMN IF EXISTS stripe_subscription_id,
  DROP COLUMN IF EXISTS market;

ALTER TABLE users
  DROP COLUMN IF EXISTS stripe_customer_id;
