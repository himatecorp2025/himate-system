# START-23.11.7 — Partner Portal Closure Audit

Release contract: `0.8.32-start-23.11.7`

## Objective

Close the Partner Portal as an end-to-end product surface rather than a collection of frontend screens. Every interactive Portal control must terminate in a real permission-checked backend/API path, authoritative persistence or an intentional read-only service, with tenant isolation and predictable failure handling. The closure also treats latency as a product requirement: independent work is parallelized, avoidable database round-trips are removed, and indexed access paths are required for Portal-critical persistence.

## Frontend closure

- The Portal shell, Overview, Modules, Results, Billing, Company, Design, Users and Notification Center are included in the audit.
- Every principal frontend API family is mapped to the Partner API gateway and the public OpenAPI contract.
- Primary dashboard data is allowed to render before secondary Billing/User/Design panels finish loading.
- Secondary independent reads start concurrently with the dashboard request.
- Mutation success is followed by authoritative readback through the existing Portal reload contract.
- Permission-gated navigation and actions remain fail-closed.
- Workspace/default-module personalization remains presentation-only and cannot mutate canonical route, API, permission, billing, workflow or entitlement identity.

## Backend / API closure

- The Partner API remains the browser-facing boundary. Internal microservices stay behind authenticated internal calls.
- Company, Catalog/Marketplace, Billing/Plans/Charity, Impact, User administration, User ↔ Module access, Design/Workspace and Notifications all have concrete gateway handlers.
- Marketplace plan metadata reads are overlapped rather than performed serially.
- User-module effective policy overlaps the independent identity assignment read and Catalog entitlement read.
- Notification delivery remains tenant, optional target-user, role-permission and effective-module scoped.
- The notification SQL read path rejects unrelated target-user events before the Go visibility pass.
- OpenAPI documents the complete Partner Portal route family, including module presentation reset via DELETE.

## Database / performance closure

- The Partner Portal user list no longer performs per-user module-mode and per-user module-count queries. User data, access mode and assignment count are returned by one SQL statement.
- `identity.partner_user_modules` retains a tenant/user/module covering index for access-policy lookup.
- Notification read-all persists all currently visible notification IDs with one bulk UPSERT instead of one database round-trip per notification.
- Notification event access retains partner-delivery, target-user and module indexes.
- Workspace settings are partner-primary-keyed; module presentation is partner/module-primary-keyed and partner-indexed.
- Catalog partner module status, entitlement and commercial access paths remain indexed.
- Billing plan due-cycle and partner module selection access paths remain indexed.
- No optimization is allowed to weaken tenant isolation, RBAC, entitlement intersection, auditability or persistence semantics.

## Responsive / UX closure

- Desktop and responsive Partner Portal shells remain explicitly separated at the Portal breakpoint.
- Narrow layouts use Drawer navigation and responsive/adaptive grids instead of desktop-only fixed columns.
- Marketplace, Result, Company, Billing and User presentation grids retain phone/tablet/desktop width transitions.
- Notification Center width is constrained for narrow viewports.
- Existing START-23 responsive test suites remain mandatory; the 23.11.7 audit adds Partner Portal-specific structural responsive assertions.
- Interactive controls must retain visible failure feedback and must not silently no-op.

## Security / tenancy closure

- Partner role permissions control surface visibility and server mutations independently from module assignment.
- Effective module access remains `partner entitlement ∩ user assignment`.
- Cross-tenant user-module IDs, design assets, workspace settings and notification events remain inaccessible.
- Notification read/read-all cannot mutate hidden or cross-tenant events.
- Internal service credentials and API/provider secrets remain server-side; Partner Portal does not expose provider keys.

## Regression / release closure

- START-23.11.4 Workspace & Personalization, START-23.11.5 User ↔ Module Permissions and START-23.11.6 Notifications remain inherited mandatory gates.
- `scripts/audit_start_23_11_7.py` is a CI contract gate for frontend/API/backend/DB/performance/responsive closure.
- `scripts/smoke_start_23_11_7.sh` is a runtime Compose closure gate and is executed only after the inherited Portal smokes.
- Go vet/unit/race, Flutter analyze/browser tests, release web build and the complete Compose regression remain mandatory before merge.
- Render and Docker Compose use one synchronized release version.
- Render auto-deploy policy is unchanged; a production Blueprint/Clear build decision is made only after exact-head CI is green.

## Closure rule

START-23.11.7 is closed only when the exact branch head has green Go, Flutter and Compose evidence and the inherited START-23.11.4–23.11.6 proofs remain green. Static source presence alone is not sufficient.
