# HIMATE System

HIMATE is the central control plane for separately deployed arts-sector partner systems.

## START-01–23.6 implementation status

### START-01–08 — Control-plane foundation
- authenticated administrator control plane with explicit REST boundaries
- versioned, service-scoped PostgreSQL migrations
- partner registry, lifecycle enforcement and server-side pagination
- Partner Workspace, canonical module catalog and partner-specific entitlement/pricing
- auditable initial-license/payment/evidence gate
- activation-date anchored 30-day billing and subscription cancellation lifecycle
- responsive Flutter administration shell and hardened gateway/session boundary

### START-09–13 — Provisioning, environments, connectors and impact
- idempotent/restartable Partner Provisioning Engine
- physically isolated partner PostgreSQL database + partner-scoped role
- structure-only reference template with no Klavierhaus business data copied
- partner staging/production environment registry
- secure partner/environment Connector Protocol with one-time raw credentials
- central System Health for services, provisioning, connector and environment state
- global/partner Impact & Metrics with explicit provenance

### START-14–16 — Evidence, PDF and CMS
- dedicated Evidence service with file validation, content sniffing and SHA-256 integrity
- immutable report snapshots and branded PDF generation through the Reports service
- versioned CMS with DRAFT → PREVIEW → PUBLISHED workflow
- preview tokens, private media pipeline and publication-time SEO validation

### START-17 — Public CMS, SSR and SEO
- published CMS content is rendered by the Gateway into the **initial HTML response**
- title, meta description, canonical, robots and Open Graph metadata are server-rendered
- hidden CMS sections are removed before the HTML response is sent
- client-side CMS JavaScript remains a hydration/fallback layer, not the SEO source of truth
- CMS-backed `/sitemap.xml` and `/robots.txt`
- dedicated START-17 integration smoke verifies initial-response HTML

### START-18–19 — Audit, governance and RBAC
- central append-only audit event model
- actor, action, resource, partner, request ID and correlation ID
- redacted old/new state snapshots for sensitive mutations
- audit filtering by actor/resource/action/partner/correlation/time range
- database trigger prevents UPDATE/DELETE of audit events
- backend-authoritative roles: Platform, Operations, Finance and Reporting
- read/write/approve permission model; Flutter visibility is UX only

### START-20 — Domains and provider deployments
- separate STAGING and PRODUCTION environment state machines
- real DNS resolution and TLS certificate/version checks
- controlled CONFIGURATION → TESTING → READY_FOR_LAUNCH → LIVE gates
- provider-neutral `runtime` microservice owns deployment adapters
- Render adapter triggers and polls real Render deploys using the Render API
- provider deployment ID/status/error are persisted
- environment becomes DEPLOYED only after the provider reports READY/live
- Docker Compose CI uses a deterministic local provider adapter and never triggers production deploys

### START-21 — Encrypted backups and verified recovery
- dedicated private `backups` microservice owns backup orchestration, retention and restore verification
- restore points include the isolated partner PostgreSQL database, partner media namespace and redacted configuration state
- restore artifacts are encrypted with chunked AES-256-GCM before durable backup storage
- CI/development uses an isolated local adapter; production uses the `render_disk` adapter backed by a dedicated Render persistent disk
- backup jobs and partner-scoped retention/scheduling policies are durable in PostgreSQL
- every successful restore point automatically queues a mandatory restore test
- restore verification re-opens the durable backup artifact, validates ciphertext/component checksums, restores PostgreSQL into a scratch database and validates media/configuration integrity
- recoverability is VERIFIED only when the latest restore point is READY and its latest restore test PASSED
- Gateway RBAC/audit, System Health and the System & Operations UI include backup/recoverability controls

### START-22 — Klavierhaus Data Connector & privacy/retention contract
- one-way Klavierhaus -> HIMATE data flow through an explicit Connector Protocol v1 boundary
- machine-readable allowlist for exactly 38 Klavierhaus functional modules
- Node.js Klavierhaus export adapter remains implementation-independent from the Go HIMATE receiver
- HMAC-SHA-512 signed requests, SHA-512 payload/data integrity, five-minute timestamps and nonce replay protection
- retained Connector business payloads use AES-256-GCM envelope encryption with per-record data keys and a versioned runtime-only master-key keyring
- partner/environment identity is derived only from the connector credential
- 1 MiB/250-item bounded batches with dataset/field validation and fail-closed sensitive-field rejection
- idempotent batch/item ingestion and SHA-512 reconciliation with SYNCED / OUT_OF_SYNC state
- approved numeric KPIs reuse the existing Impact/Reports pipeline
- accepted START-22 Connector and routed Impact data use the unified `HIMATE_7Y` product retention policy
- legal holds and mandatory privacy deletions synchronize between Connector and routed Impact observations
- System Health and the bilingual Flutter System & Operations area expose Connector Protocol, sync, coverage, retention and security state

START-22 Connector encryption runtime configuration:
- `HIMATE_CONNECTOR_DATA_MASTER_KEY_B64`: required production secret; standard-base64 encoding of exactly 32 random bytes.
- `HIMATE_CONNECTOR_DATA_KEY_VERSION`: active non-secret key version label (for example `v1`).
- `HIMATE_CONNECTOR_DATA_PREVIOUS_KEYS_JSON`: optional runtime secret JSON object mapping previous key versions to their base64 32-byte keys during controlled key rotation.
- New records always use the active key version. Older key versions remain decryptable only while their previous keys are present. Never commit any real encryption key.

## System-owner identity and localization baseline
- one durable `system_owner` account controls user creation/access administration
- the owner authority is data-driven and is never hardcoded to a person's name
- self-service profile: name, job title, phone, time zone and preferred locale
- per-user `en_US` / `hu_HU` preference with centralized Flutter translation keys
- secure self-service password change with current-password verification
- password changes rotate session version and invalidate older sessions
- Platform Admin ownership cannot be delegated through the normal user-management API

## Architecture decision
The original START blueprint recommended beginning as a modular monolith. HIMATE intentionally uses independently deployable Go microservices in containers instead. The decision, rationale, scaling constraints and consequences are recorded in:

- `docs/adr/0001-containerized-microservice-control-plane.md`
- `docs/adr/0002-public-cms-server-rendering.md`
- `docs/adr/0003-provider-deployment-adapter.md`
- `docs/adr/0004-encrypted-offsite-backup-and-restore-verification.md`
- `docs/adr/0005-klavierhaus-one-way-data-connector.md`

The architecture remains microservice/container based. It is **not** being collapsed back into a monolith.

## Current containerized service topology
- Gateway / Identity + Flutter SPA + server-rendered public marketing frontend
- Partner service
- Module Catalog service
- Billing service
- Payments service
- Contact service
- Provisioning Engine
- Environments service
- Connector service
- System Health service
- Impact & Metrics service
- Evidence service
- PDF Reports service
- CMS service
- Storage service
- Backups / recoverability service
- Runtime / deployment-provider adapter service
- Notifications service
- PostgreSQL control-plane database plus isolated partner databases
- isolated CI/development backup volume; dedicated Render persistent backup disk in production

## Security and performance baseline
- HttpOnly SameSite=Strict administrator session cookie; Secure in production
- separate HttpOnly Partner Portal cookie scoped to `/partner`; tenant ID is session-derived
- same-origin mutation protection and backend-authoritative authorization
- unique system-owner user administration
- session-version invalidation after password changes
- authenticated private service-to-service traffic
- partner Connector bearer identity isolated from administrator sessions
- bounded JSON/file inputs and strict JSON decoding
- redacted append-only governance audit
- non-root distroless runtime containers
- independent Docker images and service boundaries
- HTTP connection pooling for internal/provider calls
- partner business databases physically separated from the HIMATE control-plane DB
- START-22 retained Connector payloads encrypted at rest with AES-256-GCM envelope encryption
- Partner Portal role namespace is non-interoperable with HIMATE administrator roles
- server-paginated partner reads and bounded page-level aggregation
- encrypted durable restore artifacts with mandatory restore verification
- Go race tests, Flutter browser tests, Docker Compose health and end-to-end START-01–23.6 smoke tests in CI


### START-22.1 — Internal Control Plane Completion
- dedicated top-level Modules control plane with source/release identity, module groups and global catalog pricing
- directed dependency/integration relationships and module-to-Impact metric mappings
- partner-usage visibility from the module registry
- database-backed custom HIMATE roles with backend-authoritative permission matrices
- protected System Owner boundary remains non-delegable
- dedicated Notifications microservice with permission-aware unread/read state and audit-derived events
- Administration includes the authoritative HIMATE company/issuer/bank profile
- shared responsive KPI grid fixes stacked summary cards across Partners, Partner Workspace and Licensing & Finance
- explicit SEO settings + SEO audit acceptance coverage before START-23


### START-22.2 — Partner Portal
- separate tenant-scoped partner identity/session boundary under `/partner`
- Partner Portal roles are isolated from HIMATE administrator roles
- responsive partner self-service UI for Overview, Modules, Results, Billing, Company and Users
- tenant identity is derived only from the authenticated partner session
- partner sessions cannot authenticate to the HIMATE administrator API
- module activation reuses the authoritative Catalog and enforces availability, dependency and conflict rules
- active modules reuse the Billing 30-day subscription model with end-of-period cancellation
- Impact and billing data are read from the existing authoritative services
- company self-service is allowlisted and cannot change lifecycle, global pricing, provisioning or platform controls
- partner-user role/status changes rotate session versions; last active Owner is protected
- partner mutations reuse the central append-only audit log

### START-22.3 — Commercial Automation & Partner Website Integration
- explicit commercial agreement → activation invoice → payment evidence → PAID license → provisioning gate
- immutable partner/module price snapshot for every activation-anchored 30-day period
- point-in-time Catalog price resolution and missed-period backfill
- itemized BASE_SERVICE and MODULE invoice ledger
- immutable billing event stream for commercial, period, renewal, cancellation and invoice lifecycle
- Partner Website Adapter with domain allowlist, capabilities and AGGREGATED_ONLY privacy mode
- connector-credential-bound commercial-state contract for entitlements/configuration
- no partner website rewrite; existing sites integrate through Connector Protocol endpoints
- Partner Workspace shows commercial readiness, billing events and website-adapter configuration

### START-23 — Responsive & Functional QA
- full desktop/tablet/mobile responsive quality gate across the existing administration and Partner Portal surfaces
- explicit small-phone, phone, tablet, compact-laptop, laptop and desktop viewport matrix
- short-height login fallback, responsive header/action/dialog behavior and adaptive 1/2/4-column KPI layouts
- long partner/company/domain/status values are bounded or allowed to wrap without horizontal overflow
- record-card policy remains the responsive alternative to raw administrative data tables
- critical English/Hungarian consistency gaps from START-22.3 and the Render-native backup correction are closed
- dedicated functional route-matrix smoke covers every primary workspace plus empty, pagination and failure states
- existing privacy and concurrent-load audits remain mandatory regression gates
- START-24 Security Acceptance remains a separate final gate

### START-23.1 — Functional Contract Reset & Complete UI Inventory
- reopens product acceptance after the START-23 route-oriented QA proved insufficient for mutation completeness
- machine-readable inventory maps current UI actions and system automations to API, backend owner, persistence, audit, localization state, E2E proof requirement and closure phase
- every Flutter POST/PUT/PATCH/DELETE/multipart mutation must be represented in the contract matrix
- explicit blockers cover placeholders, mock Dashboard data, CMS/design product gaps, dynamic EN/HU schema gaps, subscription-state inconsistency, production billing scheduling and missing payment-provider integration
- representative mutation canary proves write → persistence → readback → authorization/audit behavior instead of GET-only route reachability
- START-24 is blocked until the complete START-23.2–23.12 closure program passes

### START-23.2 — Partner × Module Commercial Control Plane & Individual Pricing
- central Partner × Module matrix with partner/module perspectives, filters and responsive cards
- module defaults plus partner-specific 30-day recurring-price overrides
- module defaults plus partner-specific one-time activation-fee overrides
- effective-dated recurring-price and activation-fee history with actor/reason traceability
- current immutable Billing period displayed beside configured Catalog pricing
- exact next-period price resolved at each subscription boundary through a batched internal Catalog point-in-time quote
- partner visibility and base-service inclusion remain explicit commercial controls
- legacy duplicate module create/edit controls removed from Licensing & Finance
- end-of-period cancellation semantics remain deliberately assigned to START-23.3

### START-23.3 — Authoritative Period-End Cancellation State Machine
- Billing is the single lifecycle authority for module subscriptions: `ACTIVE` → `CANCEL_PENDING` → `INACTIVE`
- admin and Partner Portal use the same Billing cancellation command
- an active paid-period entitlement cannot be changed directly to `NOT_LICENSED` through Catalog administration
- current 30-day access and immutable price snapshots survive until the exact period boundary
- cancellation can be withdrawn before the boundary; pending/inactive subscriptions have no false next-period quote
- the daily Render Billing cron closes exact module boundaries and safely catches up missed runs idempotently
- Catalog performs final entitlement deactivation only from Billing's internal lifecycle path
- payment-provider charging and settlement remain START-23.4

### START-23.4 — Provider-Backed Activation & Recurring Payments
- isolated `payments` microservice owns provider customer/payment-method profiles, charge attempts, signed webhook verification and provider reconciliation
- Billing remains the source of truth for activation-license and invoice financial state
- administrator-entered paid amount/date/reference/verifier can no longer create a PAID activation license
- activation-license collection is an idempotent Billing command and PAID is applied only after a verified provider settlement
- recurring 30-day invoices create idempotent off-session provider attempts when partner autopay is enabled
- Stripe production adapter uses runtime secrets only; deterministic mock provider is restricted to local/CI acceptance
- webhook reconciliation verifies signature, timestamp, provider payment ID, amount and currency and preserves retryability until Billing settlement succeeds
- exact duplicate provider events are idempotent; event-ID payload conflicts fail closed
- START-23.4 acceptance is `docs/START-23.4_ACCEPTANCE.md`, `scripts/audit_start_23_4.py` and `scripts/smoke_start_23_4.sh`

### START-23.5 — Dynamic Bilingual Business Model
- dynamic business records now persist independent English and Hungarian labels/descriptions instead of relying on static UI translation alone
- shared backend locale resolution order: `locale` query → `X-Himate-Locale` → `Accept-Language` → `en_US`
- partner categories persist `name_en/name_hu`
- module groups and modules persist bilingual labels; modules also persist bilingual descriptions
- Impact metric definitions persist bilingual labels/descriptions and summary read models resolve them per locale
- custom RBAC roles persist bilingual labels/descriptions while stable `role_key` and permission semantics remain language-neutral
- Flutter sends the active locale on API requests and clears cached dynamic data on locale changes
- category, module-group, module, metric-definition and custom-role editors capture both languages
- START-23.5 acceptance is `docs/START-23.5_ACCEPTANCE.md`, `scripts/audit_start_23_5.py` and `scripts/smoke_start_23_5.sh`

### START-23.6 — Administration, Identity & Business CRUD Completion
- tokenized administrator Forgot Password is implemented with random one-time tokens, SHA-256-only persistence, expiry, replay rejection, rate limiting and central audit
- successful reset rotates `session_version`, invalidating every previously issued administrator session
- administrator email/role/status/password mutations also rotate `session_version`
- the unavailable SSO placeholder has been removed from the login UI; no authentication control is exposed without a real provider
- custom-role changes are backend-authoritative and affect existing sessions on the next permission check
- administrator create/edit/suspend, partner create/edit/lifecycle, Partner Portal user create/edit, profile/password, HIMATE company profile, Contact Leads and notification read/read-all now have dedicated Compose mutation proof
- production Gateway declares runtime-only SMTP/reset delivery configuration
- release contract version is `0.8.10-start-23.6`
- START-23.6 acceptance is `docs/START-23.6_ACCEPTANCE.md`, `scripts/audit_start_23_6.py` and `scripts/smoke_start_23_6.sh`

### Horizontal-scaling note
The service boundaries and containers allow independent scaling, but high-load production still requires shared/distributed implementations for concerns that are currently process-local, especially login throttling and any durability-sensitive asynchronous buffering. Those are explicit scaling gates rather than reasons to return to a monolith.

## Acceptance evidence
- `docs/START-04-08_ACCEPTANCE.md`
- `docs/START-09-13_ACCEPTANCE.md`
- `docs/START-14-15_ACCEPTANCE.md`
- `docs/START-16_ACCEPTANCE.md`
- `docs/START-17_ACCEPTANCE.md`
- `docs/START-18-19_ACCEPTANCE.md`
- `docs/START-20_ACCEPTANCE.md`
- `docs/START-21_ACCEPTANCE.md`
- `docs/START-22_ACCEPTANCE.md`
- `docs/START-22.1_ACCEPTANCE.md`
- `docs/START-22.2_ACCEPTANCE.md`
- `docs/START-22.3_ACCEPTANCE.md`
- `docs/START-23_ACCEPTANCE.md`
- `docs/START-23.1_ACCEPTANCE.md`
- `docs/START-23.2_ACCEPTANCE.md`
- `docs/START-23.3_ACCEPTANCE.md`
- `docs/START-23.4_ACCEPTANCE.md`
- `docs/START-23.5_ACCEPTANCE.md`
- `docs/START-23.6_ACCEPTANCE.md`
- `docs/START-23.1_FUNCTIONAL_MATRIX.json`
- `docs/START-23.1_SURFACE_INVENTORY.md`
- `docs/ARCHITECTURE.md`
- `docs/openapi.yaml`

START-22 through START-23.2 remain protected by their historical acceptance suites. START-23.3 remains protected by its lifecycle acceptance suite. START-23.4 additionally protects provider-backed activation and recurring collection. START-23.5 protects the dynamic bilingual data model. START-23.6 protects Administration, Identity and core-business CRUD with session-invalidation and password-reset mutation evidence. A dedicated START-23.1–23.6 cross-phase audit is required before START-23.7. Final live-provider proof remains reserved for START-23.12. START-24 Security Acceptance remains blocked until START-23.7–23.12 close the remaining matrix blockers.
