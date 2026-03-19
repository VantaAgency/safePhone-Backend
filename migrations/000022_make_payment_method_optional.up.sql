ALTER TABLE payments
  ALTER COLUMN payment_method DROP NOT NULL;

UPDATE payments
SET payment_method = NULL
WHERE provider = 'dexpay'
  AND payment_method IN ('hosted_checkout', 'wave', 'orange_money', 'free_money', 'card');
