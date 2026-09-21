# HIMATE control-plane architecture — START-01–21

```text
Browser / Admin / Search crawler
  |
  v
HIMATE Gateway / Identity
  |-- Flutter administration SPA
  |-- Server-rendered public CMS/SEO HTML
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
  +-- private CMS Service
  +-- private Storage Service
  +-- private Backups Service
  |       +-- Local offsite adapter (CI/dev)
  |       +-- S3-compatible offsite adapter (production)
  +-- private Runtime / Deployment Provider Service
  |       +-- Local adapter (CI/dev)
  |       +-- Render adapter (production)
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
  |    +-- cms
  |    +-- storage
  |    +-- backups
  |    +-- runtime
  |
  +-- isolated partner PostgreSQL databases
       +-- Partner A DB + partner-scoped DB role
       +-- Partner B DB + partner-scoped DB role
       +-- ...
```

## Architecture decision: containerized microservices

HIMATE intentionally retains the independently deployable Go-service topology instead of reverting to the original blueprint's modular-monolith recommendation. The formal decision is ADR-0001.

Each domain is an independently buildable Go binary and Docker image. The public Gateway is the administrative/browser ingress. Private control-plane services require the shared internal service credential. Runtime images are non-root/distroless where applicable.

The HIMATE services currently share the control-plane PostgreSQL instance while owning domain schemas. Partner business databases remain physically isolated.

### Scaling model
The topology permits independent horizontal/vertical scaling of the hot services, isolates provider failures, and prevents partner-runtime operations from requiring a Gateway rebuild. HTTP clients use connection pooling and explicit timeouts.

Containerization alone is not treated as proof of high-load readiness. Process-local state such as the current login-attempt cache must be externalized to a shared store before multiple Gateway replicas are used for global rate enforcement. Durability-sensitive queues must likewise move to durable infrastructure when their throughput/durability requirements exceed the current in-process audit queue.

## Identity, profile and user administration

The bootstrap account becomes the durable `system_owner` only when no owner exists. Startup does not overwrite a user's changed password or profile.

The owner is the sole account permitted to create or modify HIMATE administration users. Authorization is based on the owner flag and backend permissions, never the person's display name.

User accounts persist:
- name/email
- roles
- active state
- system-owner status
- preferred locale (`en_US` or `hu_HU`)
- time zone
- job title / phone
- session version

Password changes verify the current password, rotate the session version and issue a fresh current session.

## Localization

Flutter has one central key map with `en_US` and `hu_HU` locales and standard Flutter localization delegates. User preference is persisted by Identity. Login can choose an anonymous locale, while an authenticated user's persisted preference is authoritative.

This establishes the required key-based localization architecture; domain screens can migrate visible copy into the same key map without adding a translation service or a new network hop.

## Public CMS and SEO (START-17)

The CMS service remains private. The Gateway calls the published CMS endpoint with the internal service credential and renders the approved public HTML server-side.

For public marketing routes, the initial HTML response includes:
- published title
- meta description
- canonical
- index/noindex
- Open Graph title/description/url/image
- published section heading/body/CTA/media
- hidden-section removal

If CMS is unavailable, the Gateway serves the source-controlled static fallback and sets `X-Himate-SSR: static-fallback`. Published SSR responses use `X-Himate-SSR: published`.

`/sitemap.xml` is generated from the published CMS manifest. `/robots.txt` points crawlers to that sitemap.

ADR-0002 records why this rendering stays at the Gateway boundary rather than introducing a separate frontend service.

## Audit and governance (START-18)

Every authenticated mutating Gateway request generates a central audit event with:
- actor and roles
- semantic action
- HTTP method/path and resource
- partner ID where applicable
- request ID and correlation ID
- success/failure status
- redacted old/new state
- duration and timestamp

Password/token/secret/cookie/credential fields are recursively redacted. User/profile mutations have authoritative pre-mutation snapshots. The database rejects UPDATE and DELETE on `identity.audit_events`.

Audit reads support actor, resource, action, partner, correlation, method, outcome, free-text and RFC3339 from/to filtering.

## RBAC (START-19)

The backend is authoritative. Roles:
- `platform_admin`
- `operations_admin`
- `finance_admin`
- `reporting_admin`

Permissions use `resource.read`, `resource.write` and `resource.approve`. Approval-grade operations include CMS publish/rollback, provisioning runs, deployment/launch, evidence verification and licence approval.

User administration is additionally gated by `system_owner`, so a normal role assignment cannot accidentally delegate owner authority.

## Domains and provider deployments (START-20)

`environments` owns the business deployment/lifecycle state machine. `runtime` owns provider-specific deployment mechanics.

```text
Environments service
      |
      | internal deployment contract
      v
Runtime provider service
      |
      +-- local adapter --------> CI/dev deterministic READY
      |
      +-- Render adapter -------> Render Deploy API
                                  | trigger deploy
                                  | persist deploy ID/status
                                  | poll deploy status
                                  v
                             READY / DEPLOYING / FAILED
```

The Environments service does not mark a release DEPLOYED on HTTP acceptance alone. It waits for Runtime/provider state `READY`. Production lifecycle gates therefore consume provider-authoritative deployment state.

DNS verification uses resolver lookups. TLS verification performs a real TLS connection and certificate validation. Production LIVE remains blocked until deployment/runtime/domain gates are satisfied.

ADR-0003 records the provider boundary.

## Backups and verified recovery (START-21)

Backup orchestration is isolated in the private `backups` microservice. It does not run long backup or restore work inside Gateway requests.

```text
Partner PostgreSQL DB -- pg_dump --+
                                    |
Partner Storage -- archive stream --+--> manifest + SHA-256
                                    |          |
Partner/env/connector config -------+          v
                                          tar.gz package
                                               |
                                               v
                                      chunked AES-256-GCM
                                               |
                                               v
                                     Offsite provider adapter
                                      |                 |
                                      | local           | S3-compatible
                                      v                 v
                                    CI/dev          production
                                              
Offsite restore point
       |
       +--> ciphertext hash + AES-GCM authentication
       +--> manifest/component checksum verification
       +--> pg_restore into temporary scratch DB
       +--> restored partner identity verification
       +--> safe media extraction
       +--> configuration parse/identity verification
       v
  PASSED / FAILED recoverability proof
```

Restore-point and restore-test queues are durable PostgreSQL state. Workers use `FOR UPDATE SKIP LOCKED`, so process restarts do not lose queued work and multiple workers do not claim the same job.

Every successful restore point automatically queues a restore test against the **offsite copy**. A partner is reported `VERIFIED` only when the latest restore point is `READY` and its corresponding restore test is `PASSED`.

Restore artifacts contain:
- the isolated partner database in PostgreSQL custom dump format,
- the partner media namespace,
- partner/environment/connector configuration with secret-bearing keys recursively removed,
- a manifest containing component SHA-256 hashes and byte sizes.

Artifacts are encrypted with chunked AES-256-GCM. The encryption key is runtime-secret configuration. CI/development uses a separate local offsite Docker volume; production uses the S3-compatible HTTPS adapter with SigV4 credentials. The application/storage disk is not treated as a production offsite boundary.

Partner backup policy persists retention days, maximum restore-point count, automatic interval and enabled state. Pruning deletes the remote object before metadata is marked `EXPIRED`.

ADR-0004 records the backup/offsite/restore-verification boundary.

## Provisioning and data isolation

Provisioning is a persisted state machine. It validates partner lifecycle and the initial-license gate before changing the partner to `PROVISIONING`. Completed steps are durable and skipped on retry.

Reference-template operations are structure-only and never copy Klavierhaus business/customer/financial/media data.

## Connector Protocol

HIMATE never performs cross-tenant SQL through the Connector. Credentials are partner+environment scoped. Raw bearer credentials are returned only on generation/rotation and only hashes are persisted.

## Evidence, reports and storage

Evidence is validated and SHA-256 checked. PDF report jobs freeze immutable snapshots. Storage is an abstraction used by evidence/report/CMS media. Partner and control-plane data boundaries remain explicit.

## Deployment topology

Local/CI uses `docker-compose.yml`, the Runtime `local` deployment provider and a separate local backup offsite volume. Render topology is declared in `render.yaml`; all Git auto-deploy remains disabled and production deployment is controlled. Production Backups uses an S3-compatible HTTPS offsite provider and runtime-injected encryption/storage credentials.

The Render Runtime service is configured for the `render` provider and receives `RENDER_API_KEY` / optional default service ID as secrets. Provider credentials never live in source control.
