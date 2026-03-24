CREATE TABLE IF NOT EXISTS employee_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    suspended_reason TEXT,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_employee_profiles_status
        CHECK (status IN ('active', 'inactive', 'suspended'))
);

CREATE INDEX IF NOT EXISTS idx_employee_profiles_org_status
    ON employee_profiles(org_id, status, updated_at DESC);

INSERT INTO employee_profiles (user_id, org_id, status, created_at, updated_at)
SELECT u.id, u.org_id, 'active', now(), now()
FROM users u
WHERE u.role = 'employee'
  AND u.deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING;
