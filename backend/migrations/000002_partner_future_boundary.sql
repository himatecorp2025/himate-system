-- Reserved boundary for START-04+ partner entities.
-- This file intentionally contains no partner business schema yet. Keeping the
-- boundary explicit prevents START-01/03 from silently implementing later scope.

BEGIN;

CREATE TABLE IF NOT EXISTS himate_schema_capabilities (
    capability_key      TEXT PRIMARY KEY,
    status              TEXT NOT NULL CHECK (status IN ('implemented', 'reserved')),
    notes               TEXT NOT NULL DEFAULT '',
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO himate_schema_capabilities (capability_key, status, notes)
VALUES
    ('start.core', 'implemented', 'START-01/03 core schema contract'),
    ('partners.registry', 'reserved', 'START-04+'),
    ('modules.catalog', 'reserved', 'START-06+'),
    ('provisioning.engine', 'reserved', 'START-09+')
ON CONFLICT (capability_key) DO NOTHING;

COMMIT;
