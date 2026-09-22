# HIMATE control-plane architecture — START-01–23.8

```text
Browser / Admin / Partner Portal / Search crawler
  |
  v
HIMATE Gateway / Identity
  |-- Flutter administration SPA
  |-- Flutter Partner Portal SPA (/partner)
  |-- Server-rendered public CMS/SEO HTML
  |
  +-- private Partner Service
  +-- private Catalog Service
  +-- private Billing Service
  +-- private Payments Service
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
  |       +-- Local filesystem adapter (CI/dev)
  |       +-- Render persistent-disk adapter (production)
  +-- private Runtime / Deployment Provider Service
  |       +-- Local adapter (CI/dev)
  |       +-- Render adapter (production)
  +-- private Notifications Service
  |
  +-- HIMATE PostgreSQL control-plane database
  |    +-- identity (admin + isolated partner identities)
  |    +-- partners
  |    +-- catalog
  |    +-- billing
  |    +-- payments
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
  |    +-- notifications
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

START-23.7 makes the Evidence boundary authoritative for commercial file proof: the Partner Workspace uploads bytes through Evidence to Storage, while Billing accepts commercial document references only after Evidence confirms same-partner ownership, file backing and a fresh SHA-256/size integrity check. Reports freeze Impact/Evidence state into immutable snapshots and regenerate PDFs only from those persisted snapshots.

START-23.8 completes the public CMS render boundary. CMS remains authoritative for versioned content, Design Guide and SEO state, while Gateway owns initial HTML composition. Published sections that are absent from source templates are emitted as escaped responsive components, and published non-template slugs are rendered inside a generic HIMATE public shell. Page and Design previews use hash-only expiring preview tokens and the same HTML rendering path. Published Design state is applied only after an explicit publish, and published SEO is inserted into the server response before JavaScript.

START-23.8 also makes visual theming a tenant-ready domain rather than coupling it to content. A design profile contains layout family, color/typography tokens, button radius and named brand-asset slots only. CMS content, partner data, modules, billing and workflows remain separate authorities. Reusable catalog profiles and partner-owned custom profiles live in `cms.design_profiles`; `cms.design_scope_state` stores the single active profile pointer. Switching a theme therefore changes one visual pointer without copying or rewriting content or mechanics. Partner-owned design media carries explicit tenant ownership, and custom profiles may reference only media from the same tenant. Public partner runtimes consume the active visual profile independently through `/public/v1/cms/partner-design/{partnerId}`. This separation allows later commercial/package rules to restrict which themes or how many custom profiles are available without changing the content or runtime model.

Published brand assets are named by surface rather than collapsed into one logo field: header wordmark, footer wordmark, favicon, app/touch icon, login logo and email/document logo. Gateway SSR consumes the public-page assets, Flutter consumes the published login/app assets, and system email consumes the published email logo, each with a built-in fallback.


START-23.9 adds role-safe executive read models without creating a new analytics monolith. Billing owns paid financial aggregation through its private `/internal/v1/analytics/dashboard` endpoint. Impact owns People Reached and the 12-month attendance series through `/internal/v1/impact/dashboard`, using the explicit `klavierhaus.events.attendance.attendee_count` metric. Gateway composes those read models with Partner and Catalog summaries.

Recent Activity remains derived from the immutable Identity audit stream. Because audit visibility depends on administrator permissions, activity is filtered after authentication and is deliberately excluded from the shared Dashboard cache. The shared cache contains role-neutral summary data only; historical-year analytics bypass that current-year cache.

Global search is implemented as permission-aware Gateway fan-out over authoritative domain services rather than a replicated search index. Before a domain is queried, Gateway verifies the caller already has that domain's read permission. This keeps search consistent with the existing backend-authoritative RBAC model and avoids cross-domain data leakage.


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

## Internal Control Plane completion (START-22.1)

START-22.1 keeps the existing microservice topology and completes the internal control plane rather than creating a second administration backend.

The Catalog service is the authoritative Module Control Plane. Module records now include business metadata plus source repository/path/ref/commit, artifact identity, version compatibility and a machine-readable manifest. Directed module relations are persisted independently from partner entitlements, allowing dependency and integration topology to be inspected without coupling it to one partner. Module-to-Impact metric mappings provide the contract later used by the Partner Portal to display module-specific results.

Identity keeps the built-in HIMATE roles as protected system roles while adding database-backed custom roles. Effective permissions are resolved by the Gateway on every authenticated request. Custom roles cannot obtain wildcard authority or System Owner approval authority, and only the System Owner can manage HIMATE users and custom role definitions.

Notifications is a dedicated private microservice. Control-plane audit events are selectively projected into permission-scoped notification events. Read/unread state is per administrator; the notification service does not become an authorization source and never replaces the Gateway audit log.

The administration frontend and future Partner Portal reuse the same domain APIs. START-22.1 does not add partner-login identity or partner self-service; those are intentionally reserved for START-22.2.

```text
HIMATE Admin SPA
      |
      v
Gateway / Identity / RBAC
      |
      +--> Catalog --------> module registry / relationships / impact mapping
      +--> Billing --------> HIMATE issuer + partner commercial state
      +--> Notifications --> permission-scoped feed/read state
      +--> existing Partners / Impact / CMS / Operations services
```

## Partner Portal and tenant identity (START-22.2)

The Partner Portal is a separate browser security plane at the same Gateway ingress. It reuses the existing domain microservices; it does not introduce duplicate partner, catalog, billing or Impact stores.

```text
Partner browser
     |
     | /partner/login + /partner/api/v1/*
     v
Gateway Partner Identity
     | session contains immutable partner_id
     | portal roles: owner/admin/billing/viewer
     |
     +--> Partners ---- allowlisted own-company fields
     +--> Catalog ----- own entitlement/price view + guarded activation
     +--> Billing ----- own summary/invoices/subscriptions
     +--> Impact ------ own aggregated results
     |
     +--> central append-only audit
```

Partner sessions use a dedicated HttpOnly SameSite=Strict cookie scoped to `/partner`. The administrator cookie and partner cookie are separate and the partner cookie is not sent to `/api/v1` administrator routes. Partner role names and permissions are independent from HIMATE RBAC and cannot resolve to `platform_admin`, custom HIMATE roles or `system_owner`.

The authoritative tenant ID is read from the authenticated partner identity on every Portal request. Client-supplied `partner_id` query/body values are never used to select a tenant. Portal access fails closed for SUSPENDED or ARCHIVED partners, including previously issued sessions.

Module self-service is implemented inside the Catalog boundary. It may change only the authenticated partner entitlement from NOT_LICENSED to ACTIVE. It cannot alter global catalog metadata, prices, source/release identity or relationships. Activation validates global availability plus `REQUIRES` and `CONFLICTS_WITH` relations before changing entitlement state.

Billing remains authoritative for renewal state. Catalog activation is synchronized through the existing Billing summary/subscription model. Cancellation sets `cancel_at_period_end`; Billing keeps the entitlement active through the already-paid 30-day period and marks it NOT_LICENSED only when the period expires.

Company self-service is allowlisted at the Gateway. Lifecycle, provisioning, environment, commercial terms and global Control Plane fields are intentionally absent from the Partner Portal write contract.

## Commercial Automation and Partner Website Integration (START-22.3)

START-22.3 closes the commercial lifecycle without creating a new business-data authority. Catalog remains authoritative for module pricing/entitlements, Billing owns commercial periods and invoices, Partners owns company/domain identity, and Connector remains the only partner-website integration boundary.

```text
Catalog price history
       |
       | effective price @ module period start
       v
Billing module_period_snapshots (immutable)
       |
       +--> MODULE invoice_items (immutable amount)
       +--> billing_events
       |
Partner base billing boundary
       |
       +--> BASE_SERVICE invoice_item
       +--> attach pending MODULE items
       v
Itemized invoice
```

A module price change during a running 30-day period cannot mutate that period. Billing asks Catalog for the price effective at the exact period start and persists one snapshot keyed by partner/module/period start. If synchronization was unavailable for one or more periods, Billing reconstructs them from activation forward using Catalog history rather than today's price.

Provisioning now consumes a stricter Billing gate:

```text
Commercial terms
   -> AGREED agreement
   -> activation-fee INVOICE evidence
   -> PAYMENT_EVIDENCE / RECEIPT
   -> verified PAID initial license
   -> provisioning allowed
```

Reference-partner waivers remain explicit documented exceptions.

Partner websites are not reimplemented. A `website_adapters` record binds the existing website, environment, allowed domains and Connector capabilities. The connector credential remains the tenant identity. `/connector/v1/commercial-state` resolves current active entitlements from Catalog and cycle/configuration state from the authoritative services. It intentionally omits partner prices, invoice bodies and payment data.

```text
Existing Partner Website
       |
       | connector credential
       v
Connector /commercial-state
       |
       +--> Catalog  (active entitlement/version)
       +--> Billing  (cycle dates only)
       +--> Partners (authoritative domain binding)
       +--> desired-state config
       |
       +--> metrics / aggregated allowlisted batches / reconciliation
```

The adapter privacy mode is `AGGREGATED_ONLY`. START-22.3 does not weaken the START-22 field allowlists, HMAC/replay controls, encrypted retained-data boundary or seven-year retention policy.

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

## Responsive and Functional QA boundary (START-23)

START-23 does not introduce a new service or business-data authority. It hardens the existing Flutter administration and Partner Portal presentation layer while preserving the accepted microservice/API boundaries.

The shared responsive layer now provides explicit mobile/tablet/desktop shell modes, compact-login fallback for short viewports, adaptive form/action/dialog primitives, bounded long-data presentation and 1/2/4-column KPI behavior. Administrative collection surfaces remain card-oriented rather than introducing raw non-responsive data tables.

The automated viewport matrix covers 320×568, 390×844, 768×1024, 1024×768, 1366×768 and 1440×900, with 1.3× text scaling on small phones. Functional QA reuses the authoritative APIs through the Gateway and adds a route-matrix smoke over the primary workspace read surfaces, pagination, empty results and a controlled 404 state.

START-23 keeps the post-START-22.3 privacy/credential-boundary and concurrent-load audits mandatory. It does not replace START-24 Security Acceptance.

## Functional contract and product-acceptance boundary (START-23.1)

START-23.1 introduces an explicit product-acceptance contract over the existing microservice architecture. It does not collapse services, duplicate business ownership, or add a new business domain.

The contract distinguishes **route reachability** from **functional completion**. A visible mutation is complete only when UI input, public API routing, backend authorization, service ownership, durable persistence/storage, readback, audit/domain events, EN/HU presentation, failure states and automated E2E proof all agree.

The authoritative machine-readable inventory lives in `docs/START-23.1_FUNCTIONAL_MATRIX.json`. CI audits every current Flutter mutation against that matrix and rejects unregistered POST/PUT/PATCH/DELETE/multipart actions. Product gaps are deliberately represented as explicit states rather than being hidden by green GET-only smoke tests.

The architecture also records three cross-service closure constraints for START-23.2–23.12:

- Catalog entitlement state and Billing subscription state must converge on one authoritative 30-day lifecycle command; an administrator must not bypass paid-period cancellation by directly forcing a catalog state.
- Billing's existing invoice-cycle code requires a production scheduler before it can be considered automatic.
- invoice generation is not payment collection; a separate provider-adapter/payment-attempt/webhook boundary is required before recurring customer charging is accepted.

START-24 Security Acceptance is therefore downstream of START-23.12, not immediately downstream of START-23.

## Partner × Module commercial control plane (START-23.2)

START-23.2 keeps service ownership explicit: Catalog owns module definitions, partner assignment, configured recurring prices, activation fees and effective-dated commercial history; Billing owns immutable 30-day subscription periods and invoice snapshots.

The administration Modules surface joins two read models without duplicating ownership. Catalog exposes the partner-by-module commercial matrix, while Billing exposes current subscription periods. For each subscription boundary Billing performs one authenticated batched internal quote request to Catalog so the displayed next-period price is resolved from the same point-in-time price history used by billing logic rather than guessed by Flutter.

Module registry defaults now include a one-time activation fee. Each partner assignment can override both recurring price and activation fee with effective dates and actor/reason history. Current paid-period snapshots remain immutable when configured future prices change.

The legacy module create/edit path in Licensing & Finance has been removed so the Modules control plane is the only administrative registry owner. Normal cancellation is intentionally not finalized here: START-23.3 makes Billing authoritative for module-subscription lifecycle. Each subscription has an explicit `ACTIVE`, `CANCEL_PENDING` or `INACTIVE` state. Admin and Partner Portal cancellation commands converge on the same Billing mutation and append the same subscription-history/Billing-event trail.

Catalog remains authoritative for assignment metadata, visibility and commercial configuration, but cannot externally terminate an active paid-period entitlement. An external `ACTIVE -> NOT_LICENSED` request is rejected; after the exact 30-day period boundary the Billing cycle uses the private internal Catalog path to complete deactivation. This preserves paid access and immutable period pricing while preventing an admin UI or API caller from bypassing the commercial lifecycle.

The subscription read model suppresses next-period quotes for `CANCEL_PENDING` and `INACTIVE` subscriptions. Therefore UI surfaces cannot display a misleading next renewal after cancellation is scheduled. Cancellation may be withdrawn before period end, returning the subscription to `ACTIVE`.

The Render Blueprint's daily `himate-30day-invoice-cycle` job executes the same Billing lifecycle engine. Daily execution is intentionally independent of partner base-fee dates: each module boundary is evaluated, missed-run catch-up is deterministic from immutable snapshots, and repeated execution at the same boundary is idempotent.

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
                                     Backup storage adapter
                                      |                 |
                                      | local           | render_disk
                                      v                 v
                                    CI/dev       Render persistent disk
                                              
Durable restore point
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

Every successful restore point automatically queues a restore test against the **durable stored copy**. A partner is reported `VERIFIED` only when the latest restore point is `READY` and its corresponding restore test is `PASSED`.

Restore artifacts contain:
- the isolated partner database in PostgreSQL custom dump format,
- the partner media namespace,
- partner/environment/connector configuration with secret-bearing keys recursively removed,
- a manifest containing component SHA-256 hashes and byte sizes.

Artifacts are encrypted with chunked AES-256-GCM. The encryption key is runtime-secret configuration. CI/development uses a separate local Docker volume; production uses the `render_disk` adapter on a dedicated Render persistent disk mounted only into the Backups service. The backup disk is separate from the HIMATE application/storage disk. This keeps backup artifacts isolated at the service/disk level while intentionally remaining inside the Render provider failure domain.

Partner backup policy persists retention days, maximum restore-point count, automatic interval and enabled state. Pruning deletes the stored backup artifact before metadata is marked `EXPIRED`.

ADR-0004 records the backup-storage/restore-verification boundary and the Render-native production amendment.

## Provisioning and data isolation

Provisioning is a persisted state machine. It validates partner lifecycle and the initial-license gate before changing the partner to `PROVISIONING`. Completed steps are durable and skipped on retry.

Reference-template operations are structure-only and never copy Klavierhaus business/customer/financial/media data.

## Connector Protocol and Klavierhaus START-22 data boundary

HIMATE never performs cross-tenant SQL through the Connector. Credentials are partner+environment scoped. Raw bearer credentials are returned only on generation/rotation and only hashes are persisted.

START-22 adds a language-neutral one-way Klavierhaus -> HIMATE data contract. Klavierhaus remains free to use its current Node.js/Express/SQLite implementation or a future Go backend because the integration boundary is HTTPS/JSON rather than shared code or database access.

```text
Klavierhaus business DB
        |
        | 38 explicit privacy-safe collectors
        v
Klavierhaus export adapter
        |
        | Connector Protocol v1
        | bearer partner/environment identity
        | SHA-512 body/data digest
        | HMAC-SHA-512 signature
        | timestamp + nonce replay protection
        v
HIMATE Gateway -> Connector
        |
        +-- retained allowlisted START-22 record (HIMATE_7Y)
        +-- numeric KPI -> Impact (same retention/provenance)
        +-- reconciliation -> System Health
```

The Connector owns a machine-readable registry for exactly 38 Klavierhaus modules. Unknown module/dataset pairs, unknown fields, nested/free-text payloads outside the contract and secret-bearing fields fail closed. Batch identity and partner identity cannot be supplied by Klavierhaus JSON.

Signed batches are bounded to 1 MiB and 250 items. Exact retries are idempotent; conflicting reuse is rejected. Reconciliation compares per-dataset counts and aggregate SHA-512 checksums and persists SYNCED / OUT_OF_SYNC state.

All accepted START-22 records use the HIMATE product policy `HIMATE_7Y`. Routed Impact observations inherit the same retention deadline. Legal hold blocks deletion; mandatory privacy deletion is synchronized from Connector to the routed Impact copy before Connector purge. The seven-year value is a HIMATE policy rather than a claim of one universal US statutory retention period.

ADR-0005 records this boundary and the reasons for rejecting direct SQL access and raw Klavierhaus database replication.

## Evidence, reports and storage

Evidence is validated and SHA-256 checked. PDF report jobs freeze immutable snapshots. Storage is an abstraction used by evidence/report/CMS media. Partner and control-plane data boundaries remain explicit.

## START-23.4 payment-provider boundary

Payments is an independently deployable private microservice. Billing never stores Stripe API credentials and Payments never updates Billing tables directly.

```text
Admin / Billing scheduler
        |
        v
Billing -- authoritative obligation / amount / invoice
        |
        | internal charge intent + idempotency key
        v
Payments -- provider profile + attempt ledger
        |
        | Stripe REST (production) / mock adapter (CI)
        v
Payment provider
        |
        | signed PaymentIntent webhook
        v
Gateway public webhook route
        |
        v
Payments -- signature + timestamp + payment ID + amount/currency verification
        |
        | internal verified settlement
        v
Billing -- PAID / FAILED + immutable billing event
```

Production uses the Stripe adapter with runtime-only `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET`. Local/CI uses the deterministic mock adapter but still exercises the same persisted attempt, signed-webhook and Billing settlement path. Webhook events stay retryable until Billing settlement succeeds; exact processed duplicates are idempotent and event-ID payload reuse fails closed.

## START-23.5 dynamic localization boundary

Dynamic business localization is a persistence concern, not only a Flutter translation concern.

Stable technical identifiers remain language-neutral:
- category IDs/slugs;
- module-group keys;
- module keys;
- Impact metric keys;
- RBAC role keys and permission keys.

Localized metadata is stored alongside those identifiers:

```text
Partners: categories.name_en / name_hu
Catalog: module_groups.label_en / label_hu
Catalog: modules.label_en / label_hu + description_en / description_hu
Impact: metric_definitions.label_en / label_hu + description_en / description_hu
Identity: custom_roles.label_en / label_hu + description_en / description_hu
```

All participating services use the shared `services/internal/common/locale.go` contract. Resolution order is explicit query locale, `X-Himate-Locale`, `Accept-Language`, then `en_US`. APIs expose both stored variants while the legacy display field resolves to the active locale for backward compatibility.

Flutter sends `X-Himate-Locale` on API requests and invalidates API cache when locale changes so cached dynamic records cannot leak across language switches. Authorization, billing and entitlement semantics never depend on localized strings.

## START-23.6 identity and administration boundary

Identity remains owned by Gateway's `identity` schema. START-23.6 adds a server-side password-recovery contract without introducing a third-party identity dependency:

```text
reset request
  -> cryptographically random 32-byte token
  -> SHA-256 token hash persisted in identity.password_reset_tokens
  -> runtime SMTP delivery in production
  -> one-time confirmation
  -> password hash replacement
  -> identity.users.session_version + 1
  -> every prior administrator session becomes invalid
```

Reset-token plaintext exists only long enough to be delivered to the user. The database stores only the hash, expiry and used-state. Production fails closed when SMTP/reset-link configuration is unavailable. Local/CI can surface the token only while `HIMATE_ENV != production` so the real one-time flow can be acceptance-tested deterministically.

Administrator authorization continues to use database-backed roles on every request. Email, role, active-status and password changes rotate `session_version`; system-owner and last-Platform-Admin protections remain authoritative. Custom-role permission changes do not require re-login because effective permissions are resolved from the current role definition.

Partner Portal identities retain a separate cookie/session namespace and tenant claim. Their existing role/status/password mutations also rotate the Partner Portal session version.

No SSO provider is configured in this phase. Consequently the former non-functional SSO button is absent from the login surface. Provider-specific SSO must not reappear as UI until the corresponding backend authentication boundary exists.

The same START-23.6 acceptance also proves existing service ownership rather than duplicating data: Partners owns partner profile/lifecycle, Billing owns the HIMATE company profile, Contact owns persisted leads, Notifications owns per-user read state, and Gateway owns administrator/profile identity.

## Deployment topology

Local/CI uses `docker-compose.yml`, the Runtime `local` deployment provider and a separate local backup volume. Render topology is declared in `render.yaml`; all Git auto-deploy remains disabled and production deployment is controlled. The isolated `himate-payments` private service owns payment-provider connectivity and receives Stripe credentials only as runtime secrets. Production Backups uses the `render_disk` provider with a dedicated `/offsite` Render persistent disk and the runtime-injected AES-256 encryption key. No AWS/S3 endpoint or credential is required by the current production topology.

The Render Runtime service is configured for the `render` provider and receives `RENDER_API_KEY` / optional default service ID as secrets. Provider credentials never live in source control.
