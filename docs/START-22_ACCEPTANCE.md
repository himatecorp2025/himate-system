# START-22 Acceptance — Klavierhaus Data Connector & US Privacy Contract

## Scope and architecture
- [x] The primary START-22 data direction is Klavierhaus -> HIMATE.
- [x] HIMATE never receives direct SQL access to the Klavierhaus business database.
- [x] The Klavierhaus Node.js adapter and the HIMATE Go Connector communicate only through Connector Protocol v1.
- [x] The contract is implementation-language neutral and can be reused by the planned Klavierhaus Go backend.
- [x] The HIMATE Connector remains an independently deployable, containerized Go microservice.

## 38-module mapping contract
- [x] Exactly 38 Klavierhaus modules are represented in the machine-readable registry.
- [x] Every module has one stable module_key and dataset_key.
- [x] Every dataset has an explicit transfer mode, target service, cadence and schema version.
- [x] Every dataset has an explicit field allowlist.
- [x] Unknown module/dataset pairs and unknown fields are rejected fail-closed.
- [x] Passwords, sessions, secrets, tokens and payment credentials are not exportable datasets.
- [x] Klavierhaus collectors query only approved aggregate/business metadata instead of serializing raw tables.

## Connector Protocol v1
- [x] Protocol version is 1.0 and source_system is KLAVIERHAUS.
- [x] Batch ingestion is exposed at POST /connector/v1/data/batches.
- [x] Reconciliation is exposed at POST /connector/v1/reconcile.
- [x] Partner and environment identity come only from the connector credential.
- [x] Payloads are limited to 1 MiB and 250 items per batch.
- [x] Every item requires a schema version, date period, aggregation rule, source checksum and idempotency key.
- [x] Batch and item idempotency reject conflicting reuse and safely accept exact retries.
- [x] Daily reconciliation compares dataset item counts and aggregate SHA-512 checksums.

## Security
- [x] Connector credentials remain partner+environment scoped and raw credentials are returned only on generation/rotation.
- [x] Raw connector credentials are not persisted by HIMATE.
- [x] START-22 signed requests require X-Himate-Timestamp, X-Himate-Nonce, X-Himate-Body-SHA512 and X-Himate-Signature.
- [x] Request authentication/integrity uses HMAC-SHA-512.
- [x] Data and body integrity use SHA-512.
- [x] Signature comparison is constant-time.
- [x] Signed requests have a five-minute timestamp window.
- [x] Nonces are credential-scoped and replay-protected.
- [x] Connector request bodies reject nested/free-text data outside the approved scalar contract.
- [x] Secret-bearing fields remain denied even if a future registry entry is incorrectly expanded.
- [x] Retained START-22 Connector payloads are encrypted at rest with AES-256-GCM envelope encryption.
- [x] Every retained record uses a random 256-bit data-encryption key and independent nonces.
- [x] Per-record data keys are wrapped by a runtime-only versioned 256-bit master key and the raw master key is never persisted.
- [x] The keyring supports previous key versions so retained seven-year data remains decryptable after controlled key rotation.
- [x] Fresh records leave the legacy JSONB data column empty; admin reads decrypt only inside the Connector service boundary.
- [x] Pre-release plaintext START-22 records are encrypted automatically during service startup after the encryption migration.

## Routing and HIMATE reuse
- [x] Approved numeric Klavierhaus KPIs route into the existing Impact service.
- [x] Stable metric keys use the klavierhaus.<dataset>.<field> namespace.
- [x] START-22 system metric definitions are created only through a private internal endpoint.
- [x] Connector observations use PARTNER_DECLARED provenance and source_ref links back to the retained Connector record.
- [x] Reports can consume routed values through the existing Impact/Reports architecture.
- [x] Non-Impact datasets remain retained and visible through the Connector operations layer without creating a second business database.

## Unified seven-year retention
- [x] Every accepted START-22 batch, record and reconciliation uses HIMATE_7Y.
- [x] retain_until defaults to received_at + seven years.
- [x] Routed Impact observations inherit HIMATE_7Y and the same retain_until boundary.
- [x] Connector and Impact both run automatic expired-record retention workers.
- [x] Legal hold blocks deletion.
- [x] Connector legal-hold changes synchronize to routed Impact observations before the Connector mutation commits.
- [x] Mandatory privacy deletion synchronizes to routed Impact observations and is blocked by legal hold.
- [x] Retention mutation endpoints require connectors.approve at the Gateway.

The seven-year period is a HIMATE product policy for accepted START-22 data. It is not documented as a universal United States statutory minimum. Mandatory legal/privacy deletion duties and legal holds can override the default policy.

## System Health and UI
- [x] Connector partner state persists protocol_version, sync_status, last_data_sync_at and last_reconciliation_at.
- [x] System Health consumes reconciliation-aware Connector state.
- [x] The Flutter System & Operations area includes a Klavierhaus Data Connector panel.
- [x] The panel exposes connection/sync/protocol/module coverage/retention/security state without displaying secrets.
- [x] English and Hungarian localization keys are present.
- [x] Retention mutations remain approval-grade operations.

## Klavierhaus adapter
- [x] The adapter lives in server/himate-connector and is separated from ERP business route code.
- [x] It has the same 38-module registry and dataset keys as HIMATE.
- [x] It computes canonical SHA-512 dataset checksums compatible with the Go receiver.
- [x] It signs requests with HMAC-SHA-512 and sends a new nonce per request.
- [x] It supports heartbeat, five-minute system sync, hourly operational sync and daily full reconciliation.
- [x] Connector credentials are read from environment configuration and are never written to Klavierhaus SQLite.
- [x] Collector schema failures fail closed instead of silently publishing zero/partial data.
- [x] Dedicated Node.js tests verify registry coverage, privacy boundaries, signing and collector behavior.

## Automated evidence
- [x] Go unit tests cover registry invariants, canonical JSON/SHA-512, signatures, allowlists, prohibited fields, AES-256-GCM envelope integrity and key rotation.
- [x] Go vet/unit/race/build run in CI.
- [x] Flutter analyze/browser tests/release build run in CI.
- [x] Klavierhaus has a dedicated START-22 Connector Contract CI job.
- [x] HIMATE START-22 end-to-end signed batch/replay/idempotency/reconciliation/retention smoke is green.
- [x] Updated OpenAPI contract is verified in CI.
- [x] Full HIMATE START-01–22 Compose regression is green.

## Release-gate evidence
- HIMATE branch head before documentation closure: `3c1d33ad8f980573aa13f240fd5196efa44928fa`
- GitHub Actions run `35625952285`: Go, Flutter and full Compose/START-01–22 regression all SUCCESS.
- Klavierhaus dedicated `START-22 Connector Contract` job is SUCCESS on branch head `53bab1cdef7fba49e9db41ee509c940aeb7c93b6`; pre-existing legacy Klavierhaus CI failures are tracked separately from START-22.

## Evidence
- `services/cmd/connector/start22_registry.go`
- `services/cmd/connector/start22_ingest.go`
- `services/cmd/connector/start22_admin.go`
- `services/cmd/connector/main_test.go`
- `services/cmd/impact/main.go`
- `services/cmd/health/main.go`
- `services/cmd/gateway/main.go`
- `frontend/lib/start22_connector.dart`
- `frontend/lib/localization.dart`
- `scripts/smoke_start_22.sh`
- `docs/adr/0005-klavierhaus-one-way-data-connector.md`
- Klavierhaus: `server/himate-connector/*`
- Klavierhaus: `test/himate-connector.test.js`

START-22 is complete only when its dedicated HIMATE end-to-end smoke, OpenAPI gate, Go/Flutter jobs and the complete HIMATE START-01–22 Compose regression are green. Klavierhaus legacy CI failures that are proven identical to the pre-START-22 develop baseline are tracked separately from the dedicated START-22 Connector Contract gate.
