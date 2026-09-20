# START-09–13 Acceptance

This document is the release gate for the HIMATE START-09 through START-13 block. The block is not complete merely because individual services compile; every criterion below must pass in CI and in the final re-audit.

## START-09 — Provisioning Engine

- [x] Provisioning is a dedicated containerized Go microservice.
- [x] One stable provisioning job exists per partner.
- [x] Workflow steps and attempts are persisted.
- [x] Successful steps are skipped on retry, making the workflow restartable/idempotent.
- [x] A partner must be in `READY_TO_PROVISION`, `PROVISIONING`, or `CONFIGURATION`.
- [x] Initial license is checked before the lifecycle can move to `PROVISIONING`.
- [x] License gate requires `PAID + persistent commercial evidence`, or an explicitly waived reference partner.
- [x] A physically separate PostgreSQL database and isolated partner role are created.
- [x] Database credentials are derived from a dedicated master secret and are not stored as plaintext control-plane records.
- [x] Reference-template seeding copies structure only; no Klavierhaus customers, pianos, invoices, users, messages, media, finance data or statistics are copied.
- [x] An initial partner-admin invite is seeded when an admin email is supplied.
- [x] Module preset is applied both to the partner database foundation and central entitlement catalog.
- [x] A staging environment is created/idempotently ensured.
- [x] A staging connector identity is created/idempotently ensured.
- [x] Partner database readiness/identity is checked.
- [x] Successful completion moves the partner to `CONFIGURATION`.
- [x] Re-running a completed job creates no duplicate partner database or staging environment.

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

## START-12 — System Health

- [x] System Health is a dedicated containerized Go microservice.
- [x] All control-plane microservices and PostgreSQL are monitored.
- [x] Service latency, status, error and timestamp are reported.
- [x] Connector, environment, provisioning and version state are aggregated per partner.
- [x] Background snapshots are persisted for fast Partner Portfolio reads.
- [x] Production connector telemetry is preferred when both production and staging telemetry exist.
- [x] Responsive System & Operations UI exposes service health, partner health, provisioning and environments.

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

## Cross-cutting release gates

- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go build ./cmd/...`
- [ ] `flutter analyze`
- [ ] `flutter test --platform chrome`
- [ ] `flutter build web --release`
- [ ] Docker Compose topology validates.
- [ ] All microservice containers build and start.
- [ ] Gateway reports all private services healthy.
- [ ] START-01–08 regression smoke still passes.
- [ ] START-09–13 end-to-end smoke passes, including isolated partner DB, idempotent retry, staging, connector heartbeat, metric sync and central health.
- [ ] Feature-branch Vercel preview deployment remains disabled; `develop` is the deployment branch.

Only after every cross-cutting gate is green should PR #2 leave draft state and be merged into `develop`.
