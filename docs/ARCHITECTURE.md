# HIMATE control-plane architecture — START-01–15

```text
Browser / Admin
  |
  v
HIMATE Gateway / Identity / Flutter + Public Website
  |
  +-- private Partner Service
  +-- private Catalog Service
  +-- private Billing Service
  +-- private Contact Service
  +-- private Provisioning Engine
  +-- private Environments Service
  +-- private Connector Control API
  +-- private System Health Service
  +-- private Impact & Metrics Service
  +-- private Evidence Service
  +-- private PDF Reports Service
  +-- private Storage Service
  |
  +-- HIMATE PostgreSQL control-plane database
  |    +-- identity
  |    +-- partners
  |    +-- catalog
  |    +-- billing
  |    +-- contact
  |    +-- provisioning
  |    +-- environments
  |    +-- connector
  |    +-- health
  |    +-- impact
  |    +-- evidence
  |    +-- reports
  |    +-- storage
  |
  +-- isolated partner PostgreSQL databases
       +-- Partner A DB + partner-scoped DB role
       +-- Partner B DB + partner-scoped DB role
       +-- ...
```

## Containerized microservices

Each backend domain is an independently deployable Go binary and Docker image. Only the Gateway and the explicitly public Connector Protocol surface are network-facing. Administrator APIs remain session-authenticated through the Gateway. Private service endpoints require the shared internal credential. Runtime images are distroless and run as non-root.

The HIMATE control-plane services may share the HIMATE PostgreSQL instance while owning separate schemas. Database changes are service-scoped, ordered and transactional. Partner business databases do **not** share this database: START-09 creates a physically separate database and role per partner.

## Provisioning and data isolation

Provisioning is a persisted state machine. It validates partner lifecycle and the initial-license gate before changing the partner to `PROVISIONING`. Completed steps are durable and skipped on retry.

The reference-template operation is structure-only. It may create partner platform metadata, module-entitlement structure and an initial administrator invite, but it must never copy Klavierhaus customer, piano, invoice, user, chat, media, financial or statistical data.

Partner database credentials are derived from a dedicated provisioning master secret and are never written to the normal control-plane tables as plaintext.

## Environment model

A partner may have independent `STAGING` and `PRODUCTION` environment records. Each stores hostname, configuration, platform version, desired/active release, deployment status and environment status. START-10 manages the state model; the later START-20 deployment/domain block remains responsible for full production launch/deployment automation.

## Connector Protocol

```text
HIMATE Control Plane
        |
        | explicit Connector Protocol
        v
Partner Backend
        |
        v
Partner-owned isolated DB
```

HIMATE never performs cross-tenant SQL queries through the Connector. Credentials are partner + environment scoped. The raw bearer token is displayed only at generation/rotation time; only its hash is persisted.

Partner systems may report health, version, module state and approved metrics. Metric traffic is forwarded to the Impact service through an authenticated internal contract.

## System Health and performance

System Health probes the individual microservices and PostgreSQL, records latency and status, and aggregates connector/environment/provisioning state by partner. Snapshots run in the background and are persisted so Partner Portfolio reads do not synchronously fan out across every operational dependency.

Partner Portfolio remains server-paginated and performs bounded parallel aggregation for only the visible page of partner IDs.

## Impact, Evidence & Reports

Impact definitions are stable and centrally governed. Observations retain period and provenance. Allowed provenance values are `SYSTEM`, `MANUAL`, `PARTNER_DECLARED`, and `VERIFIED_DOCUMENT`.

START-14 adds a dedicated Evidence service. File-backed evidence is content-sniffed, size-limited and SHA-256 verified before its bytes are persisted through the Storage service. URL evidence is reference-only and is never fetched. `VERIFIED_DOCUMENT` observations must reference a real, VERIFIED, file-backed Evidence record with matching partner/metric boundaries.

START-15 adds a dedicated Reports service. Report jobs freeze partner scope, period, metric summaries, data sources and Evidence references into an immutable snapshot before PDF rendering. Partner, multi-partner and HIMATE Global reports can therefore be regenerated from the same snapshot without rereading live metric state. Generated PDFs are stored through the same Storage abstraction and retain SHA-256 integrity metadata.

## Billing continuity

Billing remains activation-date anchored to 30-day cycles. Initial-license payment/evidence is a hard provisioning gate. Effective-dated module pricing, cancellation-at-period-end and historical records continue to be preserved from START-01–08.

## Deployment topology

Local and CI environments use `docker-compose.yml`. Render service topology is declared in `render.yaml`. Vercel preview deployments are disabled on development branches; only `develop` is configured for automatic Vercel deployment.
