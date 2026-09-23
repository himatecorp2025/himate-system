# HIMATE System

HIMATE is the central control plane for separately deployed arts-sector partner systems.

## START-01–23.9 implementation status

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
- Go race tests, Flutter browser tests, Docker Compose health and end-to-end START-01–23.8 smoke tests in CI


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

### START-23.7 — Evidence, Impact & Reproducible Reporting
- commercial Billing documents now require same-partner, file-backed HIMATE Evidence references instead of arbitrary storage URLs
- Billing performs a fresh Storage SHA-256/byte-size integrity check through the Evidence service before accepting invoice/contract/payment evidence
- Partner Workspace uploads commercial files to Evidence/Storage before linking them to Billing
- Impact definition, observation, baseline and module-to-metric mapping workflows have full mutation/readback proof
- URL/declaration and real multipart file Evidence have end-to-end create/preview/download/integrity/verification proof
- VERIFIED_DOCUMENT observations require verified file Evidence bound to the same partner and metric
- report creation freezes an immutable snapshot, persists snapshot/PDF SHA-256 values and links included Evidence
- report regeneration renders only from the stored snapshot and does not reread later live metric changes
- historical START-22.3 and START-23.4 fixtures now use real file-backed Evidence objects
- release contract version is `0.8.11-start-23.7`
- START-23.7 acceptance is `docs/START-23.7_ACCEPTANCE.md`, `scripts/audit_start_23_7.py` and `scripts/smoke_start_23_7.sh`

### START-23.8 — CMS, Design & SEO Completion
- arbitrary CMS sections absent from source templates render as escaped responsive public components
- arbitrary published CMS slugs are served as real server-rendered public pages instead of redirecting to the landing page
- CMS Preview opens private noindex full HTML through the same renderer used by publication; raw JSON preview remains API-compatible
- checksum-backed CMS media is proven through both token-scoped preview and public published URLs
- published Design Guide settings are applied server-side to public colors, typography, button radius and bilingual navigation
- brand assets are independently assignable for header/footer wordmarks, browser favicon, app/touch icon, login logo and email/document logo; published consumers use safe built-in fallbacks
- Design Guide preview uses a 30-minute hashed token and renders the real landing website in desktop 1440, tablet 834 and mobile 390 viewport wrappers
- reusable tenant-ready design profiles separate visual theme state from CMS content, modules, billing, workflows and application mechanics
- Partner Portal exposes a design catalog plus tenant-owned custom profiles and checksum-backed tenant media
- partner theme activation changes only `cms.design_scope_state.active_profile_id`; tenant media is isolated and cross-tenant asset reuse fails closed
- public partner runtimes resolve the active skin from `/public/v1/cms/partner-design/{partnerId}`, so a theme swap requires no content migration
- global SEO & Keywords publish is proven in initial HTML, including combined keywords and Organization JSON-LD
- EN/HU sibling pages, second publish and rollback are covered by dedicated end-to-end acceptance
- release contract version is `0.8.12-start-23.8`
- START-23.8 acceptance is `docs/START-23.8_ACCEPTANCE.md`, `scripts/audit_start_23_8.py` and `scripts/smoke_start_23_8.sh`

### START-23.9 — Real Dashboard, Analytics & Global Search
- Revenue YTD is calculated from provider-settled PAID activation licenses and recurring invoices owned by Billing
- monetary totals remain grouped by original currency; no implicit FX conversion is performed
- People Reached is explicitly bound to `klavierhaus.events.attendance.attendee_count` in the Impact service
- the Program Impact chart is a real 12-month time series from Impact observations rather than a hardcoded visual
- Recent Activity is sourced from the immutable governance audit stream and filtered by the signed-in administrator's read permissions
- the shared Dashboard cache excludes permission-scoped activity and historical-year requests bypass the current-year cache
- Billing and Impact Dashboard analytics are response-filtered by `billing.read` / `impact.read`; restricted roles receive no sensitive KPI values
- global search is active across permitted authoritative domains and never broadens backend authorization
- release contract version is `0.8.13-start-23.9`
- START-23.9 acceptance is `docs/START-23.9_ACCEPTANCE.md`, `scripts/audit_start_23_9.py` and `scripts/smoke_start_23_9.sh`

### START-23.10 — System & Operations Production Closure
- Provisioning Engine prepare/execute/retry is mutation-proven and idempotent after CONFIGURATION_REQUIRED
- connector credentials remain one-time raw secrets backed by stored hashes; rotation invalidates the prior token immediately
- Partner Website Adapter remains an integration layer over the existing partner website rather than a website rewrite
- environment create/edit/deploy/DNS/TLS/launch gates are mutation-proven through the isolated Environments → Runtime boundary
- production Runtime is configured for Render and fails closed against environment-level downgrade to the local test provider
- backup policies now expose last scheduler execution and the same due-policy scheduler can be run deterministically by authorized operations
- encrypted backup creation and restore verification retain their existing production proof
- release contract version is `0.8.14-start-23.10`
- START-23.10 acceptance is `docs/START-23.10_ACCEPTANCE.md`, `scripts/audit_start_23_10.py` and `scripts/smoke_start_23_10.sh`
- live Render/DNS/TLS proof remains reserved for START-23.12

### START-23.11.1 — Module Registry & Individual Commercial Model
- the canonical Klavierhaus portfolio is represented by 38 real legacy-reference modules across exactly four primary groups: Finance & Invoicing (3), Technical Operations (16), Marketing (8), Website & Events (11)
- module publication is independent from partner entitlement; only READY modules may become PUBLISHED and unpublished modules fail closed in Partner Portal
- legacy/CUSTOM partner commercial terms remain individual contracts/quotes: activation fee, base service fee, USD minimum monthly commitment, contract currency and offer reference are versioned per partner
- the USD 1,500 minimum applies to the legacy/CUSTOM INDIVIDUAL_QUOTE model only; standard Starter/Business/Flex recurring charges are controlled by their plan prices. The obsolete fixed USD 13,000 activation-fee floor is removed
- catalog/list prices are reference values only; partner-specific contract pricing is the charging authority
- partner-module recurring and activation prices retain quote/currency history and remain tenant-isolated
- release contract version is `0.8.15-start-23.11.1`
- START-23.11.1 acceptance is `docs/START-23.11.1_ACCEPTANCE.md`, `scripts/audit_start_23_11_1.py` and `scripts/smoke_start_23_11_1.sh`
- START-23.11.2 is implemented as subscription-plan recurring billing rather than individual module-price aggregation.

### START-23.11.2 — Subscription Plans & Recurring Billing
- Starter: USD 500/month, 3 fixed HIMATE-defined modules; USD 6,000/year
- Business: USD 1,500/month, 10 fixed HIMATE-defined modules; USD 18,000 annual list price → USD 16,500 annual charge
- Flex: USD 2,500/month, up to 15 partner-selected modules; USD 30,000 annual list price → USD 22,500 annual charge
- CUSTOM remains available for individually negotiated partners such as the reference Klavierhaus account
- activation/license collection is a separate prerequisite; a plan activates only after PAID or explicit waiver
- same-frequency upgrades are immediate and collect the full plan-price difference without proration
- monthly downgrades apply on the next calendar-month day 1; annual downgrades apply at annual renewal; no refund is generated
- recurring card collection is automatic; failed recurring charges retry on day 1/day 3/day 6, then suspend service for 30 days before retention-safe operational account purge
- cure-window payment restores the prior partner lifecycle and plan entitlements; purge removes operational access while retaining legally required financial/contract/audit evidence
- Billing owns plan pricing/subscription/payment state while Catalog owns the resulting module entitlements
- module-level commercial pricing/history from START-23.11.1 is retained for future add-ons/custom contracts but is not the recurring invoice authority for standard plans
- release contract version is `0.8.16-start-23.11.2`
- acceptance: `docs/START-23.11.2_ACCEPTANCE.md`, `scripts/audit_start_23_11_2.py`, `scripts/smoke_start_23_11_2.sh`

### START-23.11.3 — Module Marketplace
- Partner Portal Modules becomes a discovery-first Module Marketplace.
- all 38 canonical HIMATE/Klavierhaus modules remain discoverable by stable name and bilingual high-level summary, including unreleased legacy-reference modules
- discovery visibility is strictly separate from execution: live use still requires PUBLISHED + READY + operational ACTIVE + tenant entitlement
- Marketplace access states are ACTIVE, LOCKED, COMING_SOON and UNAVAILABLE
- Billing remains the plan-membership authority; Gateway enriches Catalog cards with available plan and higher-plan upgrade information
- Starter/Business fixed package membership and Flex selectability are never inferred from Catalog
- locked modules remain visible to support upgrade discovery but cannot bypass PLAN_MANAGED_MODULES or direct entitlement gates
- arbitrary unpublished non-marketplace modules remain hidden, preserving START-23.11.1 fail-closed behavior
- release contract version is `0.8.17-start-23.11.3`
- acceptance: `docs/START-23.11.3_ACCEPTANCE.md`, `scripts/audit_start_23_11_3.py`, `scripts/smoke_start_23_11_3.sh`

### START-23.11.3a — Platform Secrets & Provider Readiness
- Administration now contains a system-owner controlled **Secrets & API Keys** workspace.
- supported provider credentials are explicitly allowlisted and stored encrypted with AES-256-GCM; raw stored values are never returned to the UI.
- Stripe and Render consumers prefer process environment variables, then resolve the encrypted HIMATE vault dynamically.
- missing Stripe credentials no longer terminate the Payments process: health remains available with configuration-required state while charge/webhook execution fails closed.
- audit snapshots redact submitted secret values.
- release contract version is `0.8.18-start-23.11.3a`
- acceptance: `docs/START-23.11.3A_ACCEPTANCE.md`, `scripts/audit_start_23_11_3a.py`, `scripts/smoke_start_23_11_3a.sh`

### START-23.11.3b / 23.11.3c — Superseded QA fixture approach
- these releases introduced and hardened a fixed manual-QA partner fixture
- START-23.11.3d intentionally retires that fixture in favor of testing the real New Partner onboarding path
- historical acceptance documents remain for traceability, but their fixed-fixture CI gates are no longer active

### START-23.11.3d — Real Partner Onboarding
- **New Partner** now creates the partner record and immediately registers the first Partner Portal **Owner**
- the administrator/contact email becomes the initial Partner Portal login email and the wizard requires an explicit initial password
- the fixed `ptr_himate_test_001` / `test.partner@himate.test` QA records are removed by forward migrations and are no longer recreated on startup
- Partner Login now has an explicit **Clear password** control that also terminates the active browser autofill context
- Partner Portal registry errors remain precise instead of collapsing infrastructure failures into a generic access-disabled 403
- release contract version is `0.8.21-start-23.11.3d`
- acceptance: `docs/START-23.11.3D_ACCEPTANCE.md`, `scripts/audit_start_23_11_3d.py`, `scripts/smoke_start_23_11_3d.sh`

### START-23.11.3e — New Partner Master-Data Onboarding
- fixes the deployed **New Partner** button no-op: the modal no longer waits for Module Catalog or any other supplementary backend before opening
- rebuilds New Partner as a complete company master-data workflow with legal name, brand name, registration/tax identifiers, registered office, operational contacts, website/phone/domain and internal notes
- creates the first Partner Portal Owner during the same onboarding workflow
- supports partner company-logo upload through a partner-scoped CMS asset with an explicit public logo binding and authoritative `logo_url`
- partner creation stays in `PROSPECT`; licensing, evidence, module entitlement and environment launch remain controlled follow-up workflows in the partner workspace
- supplementary onboarding failures are reported without losing an already-created core partner record
- release contract version is `0.8.22-start-23.11.3e`
- acceptance: `docs/START-23.11.3E_ACCEPTANCE.md`, `scripts/audit_start_23_11_3e.py`, `scripts/smoke_start_23_11_3e.sh`

### START-23.11.3f — Partner Category Resilience
- New Partner always exposes all six canonical system categories immediately instead of collapsing to **Other** while the live registry loads
- live/custom category rows are merged over the built-in catalog
- the open New Partner modal performs one non-blocking live category refresh
- the misleading **Category service is still loading** message is removed
- Compose acceptance verifies the API returns all six system categories and creates a partner using **Gallery / cat_003**
- release contract version is `0.8.23-start-23.11.3f`
- acceptance: `docs/START-23.11.3F_ACCEPTANCE.md`, `scripts/audit_start_23_11_3f.py`, `scripts/smoke_start_23_11_3f.sh`

### START-23.11.3g — Partner Registration Error Handling
- fixes the misleading **Display name is required** response that could actually be caused by strict JSON decode/version-skew errors
- New Partner validation and API failures are rendered inside the modal instead of a bottom-page snackbar
- the modal stays open until the authoritative partner + Portal Owner + selected logo + commercial-default setup completes
- partial onboarding resumes against the already-created partner ID with **Retry setup**, preventing duplicate partners
- preserves START-23.11.3f category resilience and all existing onboarding fields
- release contract version is `0.8.24-start-23.11.3g`
- acceptance: `docs/START-23.11.3G_ACCEPTANCE.md`, `scripts/audit_start_23_11_3g.py`, `scripts/smoke_start_23_11_3g.sh`

### START-23.11.3h — Test Partner Onboarding Hardening
- duplicate partner display names are explicitly allowed; display name is presentation data, not the unique identity key
- immutable partner IDs generate collision-free internal technical slugs even when company names are identical
- one stable onboarding request ID makes core partner creation idempotent after a lost client response
- X, Cancel and route-back dismissal are blocked once partner creation is in flight or a partial partner already exists
- Retry reconciles persisted Partner Portal Owner, partner logo and commercial terms before writing those steps again
- exact duplicate-display-name and onboarding-replay behavior is covered by Compose acceptance
- START-23.11.4 remains frozen until a real production test partner is created successfully
- release contract version is `0.8.25-start-23.11.3h`
- acceptance: `docs/START-23.11.3H_ACCEPTANCE.md`, `scripts/audit_start_23_11_3h.py`, `scripts/smoke_start_23_11_3h.sh`

### START-23.11.3i — Release Consistency & Partner Data Contract
- every deployable HIMATE application service receives the same `HIMATE_APP_VERSION`
- the shared HTTP layer publishes service/release headers and rejects version-bound internal mutations when releases differ
- gateway mutation routing preflights downstream service versions; New Partner checks Partners, Billing, CMS and Storage before core creation
- `/api/v1/health` reports `service_versions`, `release_consistent`, `version_unknown` and `version_mismatch`
- mixed releases are blocked with `RELEASE_MISMATCH` instead of surfacing misleading field-validation messages
- the Compose acceptance now proves the complete partner master-data payload survives POST -> PostgreSQL -> GET without losing legal name, brand/DBA, registration, Tax/VAT, address, website, phone or operational contacts
- Partner Portal Owner persistence remains part of the same end-to-end acceptance
- Render auto-deploy remains disabled; production release completion requires all application services to be deployed from the same release
- START-23.11.4 remains frozen until the production test partner is created successfully
- release contract version is `0.8.26-start-23.11.3i`
- acceptance: `docs/START-23.11.3I_ACCEPTANCE.md`, `scripts/audit_start_23_11_3i.py`, `scripts/smoke_start_23_11_3i.sh`

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
- `docs/START-23.7_ACCEPTANCE.md`
- `docs/START-23.8_ACCEPTANCE.md`
- `docs/START-23.9_ACCEPTANCE.md`
- `docs/START-23.10_ACCEPTANCE.md`
- `docs/START-23.11.3H_ACCEPTANCE.md`
- `docs/START-23.11.3I_ACCEPTANCE.md`
- `docs/START-23.11.1_ACCEPTANCE.md`
- `docs/START-23.11.2_ACCEPTANCE.md`
- `docs/START-23.11.3_ACCEPTANCE.md`
- `docs/START-23.1-23.6_CROSS_PHASE_AUDIT.md`
- `docs/START-23.1_FUNCTIONAL_MATRIX.json`
- `docs/START-23.1_SURFACE_INVENTORY.md`
- `docs/ARCHITECTURE.md`
- `docs/openapi.yaml`

START-22 through START-23.2 remain protected by their historical acceptance suites. START-23.3 remains protected by its lifecycle acceptance suite. START-23.4 additionally protects provider-backed activation and recurring collection. START-23.5 protects the dynamic bilingual data model. START-23.6 protects Administration, Identity and core-business CRUD with session-invalidation and password-reset mutation evidence. START-23.7 protects real commercial Evidence, Impact mutation flows and reproducible snapshot-backed reporting. START-23.8 protects full CMS/Design/SEO mutation-to-initial-HTML behavior, arbitrary pages/sections, real multi-viewport previews, multi-surface brand assets, tenant-isolated partner design profiles and logic-preserving theme swaps. START-23.9 protects authoritative Dashboard revenue/Impact analytics, audit-backed Recent Activity and permission-scoped global search. START-23.10 protects provisioning, connector credentials, Website Adapter binding, environment/provider operations and backup policy scheduling. START-23.11.1 protects the canonical 38-module registry, publish-ready lifecycle gates and partner-specific versioned commercial terms. START-23.11.2 protects plan packaging, fixed/selectable entitlements, monthly/annual recurring prices, provider-backed collection and upgrade/downgrade lifecycle. START-23.11.3 protects the 38-module discovery Marketplace, discovery/execution separation and plan-aware locked-module upsell without entitlement bypass. The START-23.1–23.6 cross-phase closure audit remains an enforced CI gate during later work. Final live-provider proof remains reserved for START-23.12. START-24 Security Acceptance remains blocked until START-23.7–23.12 close the remaining matrix blockers.
