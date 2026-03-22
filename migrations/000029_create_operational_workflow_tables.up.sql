CREATE TABLE IF NOT EXISTS operational_follow_ups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    entity_type VARCHAR(30) NOT NULL,
    entity_id UUID NOT NULL,
    reason TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'to_contact',
    next_action TEXT,
    last_contact_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_operational_follow_ups_entity_type
        CHECK (entity_type IN ('client', 'subscription', 'claim', 'repair')),
    CONSTRAINT chk_operational_follow_ups_status
        CHECK (status IN ('to_contact', 'contacted', 'awaiting_response', 'resolved')),
    CONSTRAINT uq_operational_follow_ups_entity UNIQUE (org_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_operational_follow_ups_org_status
    ON operational_follow_ups(org_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_operational_follow_ups_entity_lookup
    ON operational_follow_ups(org_id, entity_type, entity_id);

CREATE TABLE IF NOT EXISTS operational_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    entity_type VARCHAR(30) NOT NULL,
    entity_id UUID NOT NULL,
    body TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_operational_notes_entity_type
        CHECK (entity_type IN ('client', 'subscription', 'claim', 'repair'))
);

CREATE INDEX IF NOT EXISTS idx_operational_notes_entity_lookup
    ON operational_notes(org_id, entity_type, entity_id, created_at DESC);
