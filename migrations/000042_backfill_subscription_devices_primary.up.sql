-- Backfill each subscription's primary device (subscriptions.device_id) into
-- subscription_devices. Per-type cap counting (CountByType) and the add-device
-- modal's remaining-slot math read from subscription_devices, so the primary
-- device must be present there. Earlier SN/DEXPAY payment subscriptions only
-- set subscriptions.device_id and never inserted the join row (the US/Stripe
-- register flow already did), which under-counted caps by one for the
-- primary device's type.
INSERT INTO subscription_devices (subscription_id, device_id)
SELECT s.id, s.device_id
FROM subscriptions s
WHERE s.device_id IS NOT NULL
ON CONFLICT DO NOTHING;
