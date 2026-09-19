-- HIMATE START-01/03 PostgreSQL foundation schema.
-- The executable START build uses an isolated bootstrap memory store until the
-- PostgreSQL adapter is introduced in the next database integration step.

BEGIN;

CREATE TABLE IF NOT EXISTS himate_users (
    id                  UUID PRIMARY KEY,
    email               TEXT NOT NULL UNIQUE,
    display_name        TEXT NOT NULL,
    password_hash       TEXT NOT NULL,
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    mfa_required        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS himate_roles (
    id                  UUID PRIMARY KEY,
    role_key            TEXT NOT NULL UNIQUE,
    display_name        TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS himate_user_roles (
    user_id             UUID NOT NULL REFERENCES himate_users(id) ON DELETE CASCADE,
    role_id             UUID NOT NULL REFERENCES himate_roles(id) ON DELETE RESTRICT,
    granted_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS himate_system_settings (
    setting_key         TEXT PRIMARY KEY,
    setting_value       JSONB NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by          UUID NULL REFERENCES himate_users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS himate_audit_events (
    id                  UUID PRIMARY KEY,
    actor_user_id       UUID NULL REFERENCES himate_users(id) ON DELETE SET NULL,
    event_type          TEXT NOT NULL,
    object_type         TEXT NOT NULL,
    object_id           TEXT NULL,
    partner_id          UUID NULL,
    correlation_id      TEXT NOT NULL,
    before_state        JSONB NULL,
    after_state         JSONB NULL,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_himate_audit_events_occurred_at
    ON himate_audit_events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_himate_audit_events_actor
    ON himate_audit_events (actor_user_id, occurred_at DESC);

COMMIT;
