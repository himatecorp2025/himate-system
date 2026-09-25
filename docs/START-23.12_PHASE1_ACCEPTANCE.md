# START-23.12 — Phase 1 Production Acceptance Closure

## Scope

This closure addresses only START-23.12 Phase 1: Frontend ↔ Backend E2E mutations, module entitlement enforcement, personalization boundaries, and Partner User ↔ Module permissions.

It does not open Phase 2.

## Findings closed

### 23.12-P1-001 — degraded secondary services must not look like empty business data

Partner Portal secondary reads now preserve the last successfully loaded state when Billing, Users, Design, Plans, Charity, or related secondary reads fail.

Failures are collected into an explicit degraded-state banner. The UI no longer converts a failed subscription/invoice/user/design read into an authoritative empty list.

### 23.12-P1-002 — entitlement synchronization pending must not look like final success

Plan changes now inspect the backend response.

When Billing returns `entitlement_sync_pending=true`, the Partner Portal shows the backend warning as a warning state instead of displaying the normal final-success toast.

### 23.12-P1-003 / P1-004 — runtime module execution is backend-enforced

The gateway now owns a canonical runtime namespace:

`/partner/api/v1/runtime/modules/{moduleKey}/...`

Every request entering this namespace passes through `requirePartnerModuleExecution`.

The backend evaluates:

`partner ACTIVE + executable entitlement ∩ authenticated user module assignment`

before dispatch.

Failures are explicit:

- `403 MODULE_NOT_OWNED` when the organization does not own executable access.
- `403 MODULE_NOT_ASSIGNED` when the organization owns the module but the authenticated user is not assigned to it.

The guarded `GET /partner/api/v1/runtime/modules/{moduleKey}/access` route provides an executable acceptance sentinel for this rule. Future partner-facing business-module runtime operations must remain under the guarded runtime namespace rather than creating direct browser-facing module routes outside it.

### 23.12-P1-005 — Golden Test privilege boundary

A non-System-Owner control-plane user cannot activate `test_partner=true`, even if the user has ordinary `partners.write` permission.

The gateway rejects the privilege escalation with `403 OWNER_REQUIRED`.

The existing Partner service immutability rule for removing Golden Test mode remains unchanged.

## Warning closure

### STARTER / BUSINESS / PREMIUM package contract

The Phase 1 runtime smoke verifies the live test topology contract:

- STARTER: module_limit = 10 and exactly 10 fixed modules.
- BUSINESS: module_limit = 20 and exactly 20 fixed modules.
- PREMIUM (stable plan key `FLEX`): module_limit = null at the API boundary and selection_mode = UNLIMITED. Every current and future eligible module is included automatically.

This is deliberately runtime evidence rather than a source-code assumption because fixed module rows are persisted configuration.

### Functional matrix

`PORTAL-MODULE-ACTIVATE` no longer remains stale as `PARTIAL_PRODUCT`. Its proof now points to the Phase 1 runtime smoke and the independent runtime entitlement gate.

## Mandatory acceptance gates

Source gate:

`python3 scripts/audit_start_23_12_phase1.py`

Runtime gate:

`sh scripts/smoke_start_23_12_phase1.sh http://127.0.0.1:8080`

The full GitHub CI runs both gates after the inherited START-23.11 Partner Portal checks.

## Acceptance rule

Phase 1 may be marked PASS only when:

1. the source audit is green;
2. the runtime smoke is green;
3. inherited tests remain green;
4. no new regression appears in the full CI;
5. the Phase 1 audit is re-read against the resulting commit.

Phase 2 remains closed until those conditions are satisfied.
