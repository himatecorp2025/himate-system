# START-23.12 Phase 5 — Final Compliance, Capacity & PWA Acceptance

## Objective

Phase 5 is the final standalone phase of START-23.12. It does not replace or relax Phase 4. Every Phase 5 surface inherits the Phase 4 browser-origin, session/MFA, Gateway authority-header and signed internal-service trust boundaries.

The phase closes three remaining production-acceptance areas:

1. seven-year immutable compliance retention for archived partner organizations;
2. measured multi-tenant capacity at the intended 50–100 company launch scale;
3. PWA/cache behavior that cannot persist authenticated tenant or control-plane data.

## 5.1 Compliance Archives

Partner lifecycle ARCHIVED is the legal-retention boundary.

When a partner transitions to ARCHIVED, the Partners service creates a compliance snapshot inside the same SQL transaction as the lifecycle change. An operational purge also refuses to commit unless a compliance snapshot exists.

The snapshot includes partner/lifecycle, administrative audit, commercial/billing, payment-attempt, Evidence and tenant-Finance records that carry partner ownership. It explicitly excludes credential-bearing domains such as administrator/Partner Portal identity records, sessions, MFA material, platform secrets and provider payment profiles.

The compliance.partner_archives table is append-only/read-only at the database layer. UPDATE and DELETE are rejected by a PostgreSQL trigger. Each archive carries the archive actor and reason, archive timestamp, a retain-until date of at least seven years, canonical JSON evidence, and a SHA-256 integrity hash.

One final archive exists per partner because ARCHIVED is terminal.

File-backed Evidence remains physically retained as well: while an archive is inside its seven-year retention window, the Storage service rejects PUT and DELETE operations for that partner namespace. Existing objects remain readable for evidence validation, but cannot be overwritten or deleted.

### Access boundary

The browser-facing endpoint is /api/v1/archives and requires audit.read.

Gateway never exposes the service archive table directly. It converts the request into /internal/v1/archives and calls Partners through the shared signed internal transport, preserving the Phase 4 timestamped HMAC service identity in production. The archive service endpoint supports GET only.

## 5.2 100-tenant capacity acceptance

The Phase 5 runtime gate creates 100 synthetic partner organizations in the ephemeral Compose database and measures concurrent requests through the authenticated Gateway.

The acceptance exercises 100 distinct tenant detail records, portfolio reads across the 100-tenant data set, repeated Compliance Archive reads, DB connection-pool contention under concurrency, zero failed requests, bounded p95 latency, and a minimum throughput floor.

This is a production-acceptance capacity proof for the intended 50–100 company launch range, not a claim of unlimited scale. Infrastructure sizing remains independently scalable as tenant activity and data volume grow.

## 5.3 PWA and cache security

The PWA service worker is deliberately public-content only.

It may cache public static assets, but it bypasses /api/, /partner/, /app, /login, non-GET requests, and cross-origin requests. Responses marked no-store or private are never inserted into the service-worker cache. Authenticated control-plane and Partner Portal data therefore remain online-only and outside offline caches.

The worker removes obsolete cache versions during activation. The public marketing shell receives a safe offline fallback; authenticated application routes do not.

The Flutter shell uses a custom supported flutter_bootstrap.js so Flutter does not register its legacy cleanup/caching worker on the root scope. This leaves /service-worker.js as the sole active service-worker owner for the application scope.

## Phase 4 inheritance

Phase 5 acceptance requires Phase 4 audit and runtime smoke to pass first. In particular Phase 5 preserves signed HttpOnly sessions and session invalidation, production MFA for privileged identities, SameSite Strict and browser-origin mutation checks, stripping/reconstruction of HIMATE authority headers at Gateway, HMAC-signed internal calls, tenant authority derived from authenticated sessions, and existing Billing, Payments and Evidence integrity controls.

No Phase 5 fixture or load-test compatibility path is enabled in production.

## Acceptance gates

Phase 5 is PASS only when all of the following are green:

- Go vet, unit tests, race tests and builds;
- Flutter analyze, browser tests and release build;
- complete inherited START regression;
- Phase 4 security architecture and runtime acceptance;
- Phase 5 architecture/compliance/PWA audit;
- immutable archive runtime proof, including rejected SQL mutation;
- cross-site archive mutation rejection;
- authenticated 100-tenant capacity/load test;
- PWA public-asset/cache contract.

After this phase is merged, START-23.12 moves to its final cross-phase closure gate before START-24.
