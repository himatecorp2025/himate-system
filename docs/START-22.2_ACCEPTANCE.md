# START-22.2 Acceptance — Partner Portal

## Scope

START-22.2 adds the partner-facing self-service layer only after START-22.1 Control Plane acceptance. It reuses the authoritative Partners, Catalog, Billing and Impact services and does not create duplicate business data stores.

### Partner identity and tenant boundary
- separate Partner Portal identity table and role namespace
- separate HttpOnly SameSite=Strict session cookie scoped to `/partner`
- authenticated identity contains immutable `partner_id`
- every Portal domain call derives tenant identity from the session
- URL/query/body attempts to select a different partner do not change tenant scope
- partner sessions cannot authenticate to the HIMATE administrator `/api/v1` surface
- partner roles are `owner`, `admin`, `billing`, `viewer`; no Partner Portal role can become a HIMATE control-plane role
- SUSPENDED and ARCHIVED partners fail closed, including already-issued sessions

### Partner Portal surface
- `/partner/login`
- responsive `/partner/app`
- Overview dashboard
- My Modules / Available Modules
- Results & Impact
- Billing and invoice history
- Company Profile
- Partner Users

### Module self-service
- module price and availability are read from the central Catalog
- partners may activate only their own eligible modules
- `REQUIRES` relations must already be active
- `CONFLICTS_WITH` active relations block activation
- MAINTENANCE and globally unavailable modules cannot be activated
- partners cannot edit global price, source identity, dependencies, version policy or module registry metadata
- activation is historized in the existing Catalog partner-module history

### Subscription lifecycle
- module activation synchronizes to the existing Billing subscription model
- subscription periods remain activation-date anchored 30-day cycles
- cancellation is always end-of-current-period
- scheduled cancellation keeps the module active until the paid period ends
- withdrawing cancellation resumes renewal
- historical/business data is not deleted by cancellation

### Company and user self-service
- company PATCH is allowlisted to own organization identity/contact/address/logo fields
- lifecycle, commercial terms, infrastructure, provisioning and HIMATE-only fields are not partner-editable
- System Owner can bootstrap the first Partner Portal Owner from the internal Partner Workspace
- Partner Owner/Admin can manage users in their own tenant according to portal permissions
- a non-owner cannot create/assign Owner
- the last active Owner cannot be removed or deactivated
- role/status/password changes rotate session version

### Audit
- partner mutations are written to the existing append-only HIMATE audit log
- events contain partner actor identity, partner role, action and authoritative partner_id
- Partner Portal does not create an independent audit authority

## Required automated acceptance

1. Go mod tidy / vet / unit / race / build.
2. Partner identity/session/role-boundary unit tests.
3. Flutter analyze / Chrome tests / release build.
4. Render Blueprint and Docker Compose validation.
5. Full START-01–22.1 historical regression suite.
6. START-22.2 two-tenant smoke verifies:
   - two isolated partner identities and cookies,
   - Partner Portal cookie cannot authenticate to admin API,
   - query/body tenant override cannot cross tenant,
   - dependency-gated module activation,
   - billing subscription synchronization,
   - end-of-period cancellation,
   - company allowlist does not mutate lifecycle,
   - partner-user role escalation is denied,
   - portal mutations reach central immutable audit,
   - archived partner access is revoked while the other tenant remains available.
7. OpenAPI contains the Partner Portal and admin-bootstrap contracts.
8. No global Control Plane settings are writable by Partner Portal APIs.

## Definition of Done

START-22.2 is DONE only when the dedicated two-tenant acceptance smoke and the entire START-01–22.1 regression suite are green on the pull request. Merge to `develop` only after those gates pass.
