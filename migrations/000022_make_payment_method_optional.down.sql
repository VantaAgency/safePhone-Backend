UPDATE payments
SET payment_method = 'hosted_checkout'
WHERE provider = 'dexpay'
  AND payment_method IS NULL;

ALTER TABLE payments
  ALTER COLUMN payment_method SET NOT NULL;
