ALTER TABLE payments
  ADD COLUMN expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_payments_subscription_created_at
  ON payments(subscription_id, created_at DESC);

UPDATE subscriptions
SET current_period_start = NULL,
    current_period_end = NULL
WHERE status = 'pending';

UPDATE devices d
SET status = CASE
  WHEN d.status IN ('expired', 'suspended') THEN d.status
  WHEN NULLIF(TRIM(COALESCE(d.imei, '')), '') IS NOT NULL
    AND EXISTS (
      SELECT 1
      FROM subscriptions s
      WHERE s.device_id = d.id
        AND s.status = 'active'
    )
    THEN 'active'
  ELSE 'pending'
END,
updated_at = NOW()
WHERE d.deleted_at IS NULL;
