# HIMATE System

HIMATE is the central control plane for separately deployed arts-sector partner systems.

## START-01–20 implementation status

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

## Pre-START-21 identity correction
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
- Runtime / deployment-provider adapter service
- PostgreSQL control-plane database plus isolated partner databases

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
- server-paginated partner reads and bounded page-level aggregation
- Go race tests, Flutter browser tests, Docker Compose health and end-to-end smoke tests in CI

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
- `docs/ARCHITECTURE.md`
- `docs/openapi.yaml`

START-21 is intentionally out of scope until the full START-01–20 regression on this correction set is green.
