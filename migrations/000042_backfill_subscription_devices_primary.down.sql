-- No-op: this is a data backfill. We can't distinguish primary-device join
-- rows created by this migration from those the US/Stripe register flow
-- already inserted, so reversing could drop legitimate attachments. Leave
-- subscription_devices as-is on rollback.
SELECT 1;
