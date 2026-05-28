-- Stripe + multi-market columns. All additive — existing SN data continues
-- to work unchanged because the new market/currency columns get sensible
-- defaults during backfill.

-- 1) Users gain a Stripe Customer ID. NULL for SN users (DEXPAY-only).
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(120);

-- 2) Subscriptions: market label + Stripe identifiers. device_id becomes
--    nullable so US subscriptions can be created at checkout completion
--    BEFORE the user has registered their phone via /us/register-device.
ALTER TABLE subscriptions
  ADD COLUMN IF NOT EXISTS market VARCHAR(2) NOT NULL DEFAULT 'SN',
  ADD COLUMN IF NOT EXISTS stripe_subscription_id VARCHAR(120),
  ADD COLUMN IF NOT EXISTS stripe_checkout_session_id VARCHAR(120);

ALTER TABLE subscriptions
  ALTER COLUMN device_id DROP NOT NULL;

-- 3) Plans gain market + currency. Backfill: us_* plans are USD/US,
--    everything else is XOF/SN.
ALTER TABLE plans
  ADD COLUMN IF NOT EXISTS market VARCHAR(2) NOT NULL DEFAULT 'SN',
  ADD COLUMN IF NOT EXISTS currency VARCHAR(3) NOT NULL DEFAULT 'XOF';

UPDATE plans
SET market = 'US', currency = 'USD'
WHERE slug LIKE 'us_%';

-- 4) Indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_stripe_customer
  ON users(stripe_customer_id)
  WHERE stripe_customer_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_stripe_subscription
  ON subscriptions(stripe_subscription_id)
  WHERE stripe_subscription_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_market_status
  ON subscriptions(market, status);

CREATE INDEX IF NOT EXISTS idx_plans_market
  ON plans(market);
