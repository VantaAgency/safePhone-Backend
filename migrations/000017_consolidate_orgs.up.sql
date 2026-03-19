-- Create the canonical shared org if it doesn't exist
INSERT INTO organizations (id, name, slug, plan, created_at, updated_at)
VALUES (gen_random_uuid(), 'SafePhone', 'safephone', 'free', NOW(), NOW())
ON CONFLICT (slug) DO NOTHING;

-- Move all data to the shared org
UPDATE users SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id != (SELECT id FROM organizations WHERE slug = 'safephone');

UPDATE devices SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id != (SELECT id FROM organizations WHERE slug = 'safephone');

UPDATE subscriptions SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id != (SELECT id FROM organizations WHERE slug = 'safephone');

UPDATE payments SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id != (SELECT id FROM organizations WHERE slug = 'safephone');

UPDATE claims SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id != (SELECT id FROM organizations WHERE slug = 'safephone');

UPDATE partners SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id != (SELECT id FROM organizations WHERE slug = 'safephone');

UPDATE partner_clients SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id != (SELECT id FROM organizations WHERE slug = 'safephone');

UPDATE partner_commissions SET org_id = (SELECT id FROM organizations WHERE slug = 'safephone')
WHERE org_id != (SELECT id FROM organizations WHERE slug = 'safephone');

-- Clean up orphan orgs
DELETE FROM organizations
WHERE slug != 'safephone'
  AND id NOT IN (SELECT DISTINCT org_id FROM users);
