-- DEXPAY product IDs on plans (one per billing cycle)
ALTER TABLE plans
  ADD COLUMN dexpay_product_id_monthly VARCHAR(100),
  ADD COLUMN dexpay_product_id_annual  VARCHAR(100);

-- DEXPAY customer ID on users
ALTER TABLE users
  ADD COLUMN dexpay_customer_id VARCHAR(100);

-- DEXPAY subscription tracking
ALTER TABLE subscriptions
  ADD COLUMN dexpay_subscription_id     VARCHAR(100),
  ADD COLUMN dexpay_checkout_session_id VARCHAR(100);

-- Webhook event dedup table
CREATE TABLE webhook_events (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider        VARCHAR(30)  NOT NULL,
  event_type      VARCHAR(100) NOT NULL,
  provider_ref    VARCHAR(200) NOT NULL,
  idempotency_key VARCHAR(300) NOT NULL UNIQUE,
  payload         JSONB        NOT NULL,
  processed_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_events_key ON webhook_events(idempotency_key);
CREATE INDEX idx_subscriptions_dexpay ON subscriptions(dexpay_subscription_id);
CREATE INDEX idx_users_dexpay ON users(dexpay_customer_id);
