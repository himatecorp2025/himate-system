# HIMATE System

HIMATE is the central control plane for separately deployed arts-sector partner systems.

## START-01–22 implementation status

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
- restore artifacts are encrypted with chunked AES-256-GCM before offsite storage
- CI/development uses an isolated local offsite adapter; production uses an S3-compatible HTTPS adapter with AWS SigV4
- backup jobs and partner-scoped retention/scheduling policies are durable in PostgreSQL
- every successful restore point automatically queues a mandatory restore test
- restore verification re-downloads the offsite artifact, validates ciphertext/component checksums, restores PostgreSQL into a scratch database and validates media/configuration integrity
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
- PostgreSQL control-plane database plus isolated partner databases
- isolated CI/development offsite backup volume; S3-compatible offsite provider in production

## Security and performance baseline
- HttpOnly SameSite=Strict administrator session cookie; Secure in production
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
- server-paginated partner reads and bounded page-level aggregation
- encrypted offsite restore artifacts with mandatory restore verification
- Go race tests, Flutter browser tests, Docker Compose health and end-to-end START-01–22 smoke tests in CI

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
- `docs/ARCHITECTURE.md`
- `docs/openapi.yaml`

The START-22 branch is accepted only when Go vet/unit/race/build, Flutter analyze/test/release build, the full START-01–21 regression, profile/owner/locale checks and the START-22 signed Connector/reconciliation/retention smoke are all green.
