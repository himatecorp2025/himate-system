# HIMATE System

HIMATE is the central control plane for separately deployed arts-sector partner systems.

## Implemented START foundation

### START-01–08
- authenticated administrator control plane with explicit REST contract
- versioned, service-scoped PostgreSQL migrations
- partner registry, lifecycle enforcement and server-side pagination
- Partner Workspace, canonical module catalog and partner-specific entitlement/pricing
- auditable initial-license/payment/evidence gate
- activation-date anchored 30-day billing and subscription cancellation lifecycle
- responsive Flutter administration shell and hardened gateway/session boundary

### START-09–13
- idempotent/restartable Partner Provisioning Engine
- physically isolated partner PostgreSQL database + partner-scoped role
- structure-only reference template with no Klavierhaus business data copied
- partner staging/production environment registry
- secure partner/environment Connector Protocol with one-time raw credentials
- central System Health for services, provisioning, connector and environment state
- global/partner Impact & Metrics with explicit provenance
- responsive partner and control-plane UI for the new functions

## Security and performance baseline
- HttpOnly SameSite=Strict administrator session cookie; Secure in production
- same-origin mutation protection, platform-admin mutation authorization and login throttling
- authenticated private service-to-service traffic
- partner Connector bearer identity isolated from administrator sessions
- bounded JSON request bodies and strict JSON decoding
- non-root distroless runtime containers
- partner business databases physically separated from the HIMATE control-plane DB
- server-paginated partner reads and bounded page-level cross-service aggregation
- background health snapshots for fast portfolio display
- Go race tests, Flutter responsive tests, Docker Compose health and end-to-end smoke tests in CI

## Current containerized service topology
- Gateway / Identity + Flutter SPA + public marketing frontend
- Partner service
- Module Catalog service
- Billing service
- Contact service
- Provisioning Engine
- Environments service
- Connector service
- System Health service
- Impact & Metrics service
- PostgreSQL control-plane database plus isolated partner databases

See `docs/ARCHITECTURE.md`, `docs/openapi.yaml`, `docs/START-04-08_ACCEPTANCE.md`, and `docs/START-09-13_ACCEPTANCE.md`.
