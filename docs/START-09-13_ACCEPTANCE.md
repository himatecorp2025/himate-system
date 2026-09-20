# START-09–13 Acceptance

This document is the release gate for the HIMATE START-09 through START-13 block.

The 2026-09-20 post-merge re-audit reconciles the original START specification with the implementation that was merged through PR #2. Existing green CI evidence is preserved, but missing specification-level acceptance gates are explicitly left unchecked. START-14 must not begin until every unchecked item below is implemented and re-tested.

## START-09 — Provisioning Engine

- [x] Provisioning is a dedicated containerized Go microservice.
- [x] One stable provisioning job exists per partner.
- [x] Workflow steps and attempts are persisted.
- [x] Successful steps are skipped on retry, making the workflow restartable/idempotent.
- [x] A partner must be in `READY_TO_PROVISION`, `PROVISIONING`, or `CONFIGURATION`.
- [x] Initial license is checked before the lifecycle can move to `PROVISIONING`.
- [x] License gate requires `PAID + persistent commercial evidence`, or an explicitly waived reference partner.
- [x] A physically separate PostgreSQL database and partner-specific role are created.
- [x] Database credentials are derived from a dedicated master secret and are not stored as plaintext control-plane records.
- [x] Reference-template seeding copies structure only; no Klavierhaus customers, pianos, invoices, users, messages, media, finance data or statistics are copied.
- [x] An initial partner-admin invite is seeded when an admin email is supplied.
- [x] Module preset is applied both to the partner database foundation and central entitlement catalog.
- [x] A staging environment record is created/idempotently ensured.
- [x] A staging connector identity is created/idempotently ensured.
- [x] Partner database readiness/identity is checked.
- [x] Successful completion moves the partner to `CONFIGURATION`.
- [x] Re-running a completed job creates no duplicate partner database or staging environment.
- [ ] The New Partner flow is a complete provisioning wizard that captures business data, commercial/license data, required documents, Partner/System identity, environment selection and module preset before provisioning begins.
- [ ] Partner storage is actually provisioned/isolated and health-checked; a `storage_namespace` metadata value alone is not sufficient.
- [ ] Provisioning performs an actual staging deployment step and verifies deployed runtime health before the workflow is considered complete.
- [ ] Cross-tenant database isolation is enforced and integration-tested so Partner A credentials cannot connect to or read Partner B resources.

## START-10 — Partner Environments & Staging

- [x] Environment management is a dedicated containerized Go microservice.
- [x] Staging and production are represented independently.
- [x] Partner/environment uniqueness is enforced.
- [x] Hostname uniqueness is enforced.
- [x] Platform version, desired/active release, configuration, deployment status and environment status are persisted.
- [x] Partner Workspace exposes responsive environment state and editing without redesigning the approved frontend.
- [x] Production environment can be registered separately after staging validation.

## START-11 — Connector Protocol

- [x] Connector is a dedicated containerized Go microservice.
- [x] No direct partner-database access exists in the Connector service.
- [x] Credential identity is partner + environment scoped.
- [x] Raw connector credential is returned only when generated/rotated; HIMATE persists only SHA-256 token hash.
- [x] Bearer comparison is constant-time.
- [x] Staging and production connector states are isolated.
- [x] Connector supports heartbeat/version/health/module-state reporting.
- [x] Connector supports approved metric synchronization to the Impact service through an internal API contract.
- [x] Partner Workspace exposes credential metadata and explicit one-time secret handling.
- [x] The public connector protocol is routed independently from administrator-session authentication.
- [ ] HIMATE → partner downstream synchronization is implemented for entitlement state, maintenance state and approved partner configuration; the current reported-state endpoint is not a substitute for desired-state delivery.

## START-12 — System Health

- [x] System Health is a dedicated containerized Go microservice.
- [x] Control-plane private microservices and PostgreSQL are monitored.
- [x] Service latency, status, error and timestamp are reported.
- [x] Connector, environment, provisioning and version state are aggregated per partner.
- [x] Background snapshots are persisted for fast Partner Portfolio reads.
- [x] Production connector telemetry is preferred when both production and staging telemetry exist.
- [x] Responsive System & Operations UI exposes service health, partner health, provisioning and environments.
- [ ] Ongoing partner-database health is monitored after provisioning, not only checked once during initial provisioning.
- [ ] Partner domain/hostname reachability/status is monitored and exposed in central System Health.
- [ ] Connector/data-sync freshness is part of persisted partner health and visible as a first-class health signal.

## START-13 — Impact & Metrics

- [x] Impact is a dedicated containerized Go microservice.
- [x] Metric definitions have stable keys, unit, aggregation and scope.
- [x] Aggregations support `SUM`, `LATEST`, and `AVERAGE`.
- [x] Scope supports `GLOBAL`, `PARTNER`, and `BOTH`.
- [x] Provenance is explicitly one of `SYSTEM`, `MANUAL`, `PARTNER_DECLARED`, `VERIFIED_DOCUMENT`.
- [x] Metric observations retain period, source reference, recorder, timestamp and metadata.
- [x] Connector ingestion supports idempotency keys.
- [x] Global and partner summaries are available.
- [x] Responsive Impact & Reports UI allows metric definition and manual value entry.
- [x] Partner Workspace exposes partner-specific statistics.
- [x] Evidence-file management is intentionally excluded and remains START-14.
- [ ] Metric baseline support is implemented and persisted so period results can be evaluated against an explicit baseline as required by the START-13 specification.

## Cross-cutting release gates

- [x] `go vet ./...`
- [x] `go test ./...`
- [x] `go test -race ./...`
- [x] `go build ./cmd/...`
- [x] `flutter analyze`
- [x] `flutter test --platform chrome`
- [x] `flutter build web --release`
- [x] Docker Compose topology validates.
- [x] All microservice containers build and start.
- [x] Gateway reports all private services healthy.
- [x] START-01–08 regression smoke still passes.
- [x] START-09–13 integration smoke passes for the currently covered scenarios.
- [x] Feature-branch Vercel preview deployment remains disabled; `develop` is the deployment branch.

## Post-merge re-audit evidence — 2026-09-20

- PR #2 (`start-09-13` → `develop`) was merged.
- PR-head HIMATE CI run #245 completed successfully for Go, Flutter and Docker Compose.
- The Compose job successfully executed both the START-01–08 regression smoke and START-09–13 integration smoke.
- The merge commit `dab3cd641e920cb51b061655dfe3e3076e8e41c5` has a successful Vercel status.
- Critical files checked across PR head and merged `develop` have identical blobs: `docker-compose.yml`, `.github/workflows/ci.yml`, `scripts/smoke_start_09_13.sh`, `services/cmd/provisioning/main.go`, and `services/cmd/connector/main.go`.
- `vercel.json` keeps deployments disabled for all branches except `develop`.

## Synchronized status

The previous checklist overstated specification completeness because it did not include several original START requirements. After reconciliation, 65 of 74 explicit acceptance gates are currently satisfied.

**START-09–13 is not yet specification-complete. Do not begin START-14 until the 9 unchecked functional gates above are implemented, covered by integration tests, and re-audited green.**
