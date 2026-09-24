# START-23.11.5 Acceptance — User ↔ Module Permissions

## Purpose

START-23.11.5 separates **organization entitlement** from **individual user access**.

The governing rule is:

> A Partner Portal user can never receive more module access than the partner organization owns.

Effective module access is always:

`partner ACTIVE + executable entitlement ∩ user assignment`

## Access modes

Every Partner Portal user has one module-access mode:

- `ALL_OWNED`: every module currently ACTIVE + executable for the partner is available to the user;
- `SELECTED`: only the explicitly selected subset of currently owned modules is available.

Existing and new users default to `ALL_OWNED` for backward compatibility.

## Partner entitlement remains authoritative

User assignments do not create, activate, purchase or license modules.

A requested `SELECTED` module must already be ACTIVE + executable for the partner. Otherwise the update is rejected with `MODULE_NOT_OWNED`.

If a partner later loses entitlement to a previously selected module, the stored grant may remain for traceability but effective user access becomes denied immediately.

## Role permissions remain separate

Portal RBAC and module assignment are two independent dimensions:

- RBAC answers **what actions** the user may perform;
- module assignment answers **which partner-owned modules** the user may use.

`users.read` may inspect own-tenant module assignments.

`users.write` may change own-tenant module assignments.

An admin may not modify an owner; only an owner may modify another owner.

A viewer cannot grant modules to itself.

## Tenant isolation

The authenticated partner ID is authoritative.

A user ID from another partner tenant is returned as not found and cannot be read or modified through the Partner Portal.

## Marketplace behavior

The full canonical Marketplace remains visible for discovery.

Each module receives:

- `user_access_state=GRANTED` when organization entitlement and user assignment both allow use;
- `user_access_state=NOT_ASSIGNED` when the organization owns the module but the user is not assigned;
- `user_access_state=ORGANIZATION_LOCKED` when the partner itself does not currently own executable access;
- `user_executable=true|false` as the effective runtime decision.

This preserves START-23.11.3 discovery behavior while adding user-level enforcement.

## Persistence

- access mode: `identity.partner_users.module_access_mode`;
- explicit grants: `identity.partner_user_modules`;
- primary key: user + module;
- user deletion cascades explicit grants.

## UI

Partner Portal → Users exposes **Manage module access**.

The editor explains that partner subscription is the upper boundary, offers ALL_OWNED / SELECTED, and lists only partner-owned ACTIVE + executable modules for explicit selection.

Module Marketplace cards show the current user's effective assignment separately from plan access.

## Release

Release contract: `0.8.30-start-23.11.5`.

Automated evidence:
- `python3 scripts/audit_start_23_11_5.py`;
- `sh scripts/smoke_start_23_11_5.sh http://127.0.0.1:8080`;
- inherited START-23.11.3 and START-23.11.4 acceptance remains mandatory.
