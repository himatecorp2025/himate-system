# START-22.1 Acceptance — Control Plane Completion

## Scope

START-22.1 completes the internal HIMATE Control Plane before any Partner Portal work begins. It extends the existing microservice architecture instead of introducing a parallel application stack.

### Module Control Plane
- dedicated top-level **Modules** navigation
- authoritative module groups and registry
- module type, owner, source repository/path/ref/commit
- release/artifact/minimum-platform metadata
- global 30-day catalog price and availability
- directed module relationships: requires, optional dependency, integrates with, extends, conflicts with, replaces
- Impact metric mapping per module
- cross-partner module usage view
- legacy module-management UI removed from Licensing & Finance

### Administration and access
- built-in roles remain protected
- System Owner can create custom roles
- permission matrix is backend authoritative
- custom roles can be assigned to administrators
- wildcard and System Owner approval authority cannot be delegated
- custom roles cannot be deactivated while assigned
- HIMATE company/issuer/bank profile is visible and editable under Administration

### Notification Center
- dedicated private notifications microservice
- per-user read state
- INFO / WARNING / CRITICAL severity
- permission-aware feed
- unread badge, mark one read, mark all read
- audit-derived control-plane notifications
- notification service included in health and deployment topology

### UI and SEO closure
- reusable responsive KPI grid
- Partners, Partner Workspace and Licensing & Finance summary cards use the same responsive layout
- global SEO settings and SEO audit endpoints are explicitly exercised by START-22.1 smoke testing
- login/bootstrap regressions from pre-START-23 remain protected

## Security boundaries
- System Owner remains the only authority that can manage HIMATE administration users or custom roles.
- Custom roles cannot contain the wildcard permission or delegate `administration.approve`.
- Notification visibility is filtered by effective backend permission.
- Source code remains in Git/version control; HIMATE stores source identity and immutable release references rather than raw application source blobs.
- Partner business databases remain isolated. START-22.1 does not introduce cross-tenant SQL.

## Required automated acceptance

The branch is accepted only when all of the following are green:

1. Go mod tidy / vet / unit / race / build.
2. Flutter analyze / Chrome tests / release web build.
3. Render Blueprint and Docker Compose topology validation.
4. Full START-01–22 regression suite.
5. Profile / owner / locale regression.
6. START-22.1 dedicated smoke:
   - notification service health
   - global SEO + SEO audit availability
   - custom role create/update/assignment and authorization enforcement
   - module group create/update
   - technical module registry creation
   - relationship graph API
   - Impact mapping API
   - partner usage API
   - HIMATE company profile editability
   - notification event delivery/read state
7. OpenAPI contract contains the START-22.1 endpoints.
8. No START-22.2 Partner Portal functionality is merged as part of this release.

## Definition of Done

START-22.1 is DONE only after the dedicated smoke and the entire historical regression suite pass on the pull request and on the merged `develop` commit. START-22.2 may begin only after that post-merge validation is green.
