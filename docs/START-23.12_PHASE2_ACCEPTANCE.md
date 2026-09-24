# START-23.12 — Phase 2 Production Acceptance Closure

## Scope

Phase 2 closes core-platform persistence, crash recovery, reload/deep-link integrity and schema-migration traceability.

The core acceptance surface includes:

- durable Partner onboarding;
- central audit durability;
- provisioning job/step integrity and restart recovery;
- external deployment intent/idempotency;
- payment and connector idempotency inherited from earlier phases;
- browser reload/deep-link reconstruction;
- migration transactionality, checksums and rollback compatibility.

## Explicit module-scope boundary

`Client Piano Intake` and `Workshop Workflow` business runtime functionality are not part of the current HIMATE core implementation. Their canonical module identities exist in the 38-module catalog, but their business services/cards/workflows are intentionally scheduled for the post-23.12 module-build program.

Phase 2 therefore does **not** create placeholders, fake endpoints or no-op workflow handlers merely to satisfy an audit sentence.

Instead:

- their module-specific crash-recovery contracts are marked **MODULE_ACCEPTANCE_DEFERRED**;
- they cannot receive a module production-closure PASS until their real frontend + backend + DB implementation exists;
- their future module-closure audit must include the same transaction, idempotency, restart and persistence requirements defined by START-23.12 Phase 2.

This is a scope correction, not a waiver of the requirement.

## P2-001 — Durable Partner onboarding saga

The browser no longer coordinates Partner master creation, Portal Owner creation and Billing terms as unrelated client-only steps.

`POST /api/v1/partner-onboarding` persists a server-side saga before executing dependent steps.

The saga stores:

- stable request_id;
- actor;
- Partner payload;
- Partner ID after creation;
- hashed Portal Owner credential (never plaintext);
- Billing terms;
- per-step completion flags;
- error/status timestamps.

Each step is idempotent:

1. Partner service receives the same `onboarding_request_id`;
2. Portal Owner creation reconciles an existing active owner before inserting;
3. Billing terms use idempotent PUT semantics.

The frontend stores only the opaque request ID while work is incomplete. After F5/browser restart, the next onboarding action calls `/resume` for the same saga instead of generating a second Partner.

Optional logo file bytes remain browser-selected and are not stored in the saga. Losing an unuploaded optional file does not affect Partner identity, ownership or commercial integrity.

## P2-002 — Provisioning creation and restart recovery

Provisioning job creation and all canonical step rows are committed in one PostgreSQL transaction.

Non-`prepare_only` jobs are persisted as `QUEUED` before execution.

`runJob` defensively ensures the canonical step set exists before work begins.

At service startup a recovery worker scans `QUEUED` and `RUNNING` jobs, retries interrupted work with bounded backoff and retains existing SUCCESS step checkpoints.

## P2-003 — Durable external deployment intent

Before any runtime provider trigger, HIMATE writes a deterministic `runtime.deployment_intents` row.

The key is derived from Partner, environment, hostname, release, provider/service and config.

Rules:

- same request => same durable intent;
- provider deploy ID already known => reconcile instead of retriggering;
- provider result uncertain and no deploy ID => return `RECONCILIATION_REQUIRED` rather than trigger a duplicate;
- provider ID is persisted before the environment deployment record is finalized.

This converts the previous duplicate-deployment race into a fail-closed recoverable state.

## P2-004 — Durable central audit outbox

Every authenticated control-plane or Partner Portal mutation must persist an `identity.audit_outbox` intent **before** business execution.

If the durable audit intent cannot be written, the mutation is rejected with `AUDIT_DURABILITY`.

Normal completion atomically:

1. appends the immutable `identity.audit_events` result;
2. removes the outbox row.

Gateway startup recovers leftover intents as append-only `*_INTERRUPTED` audit events. A SIGKILL can therefore leave an explicitly interrupted/uncertain audit result, but cannot silently erase the existence of the mutation attempt.

## P2-006 / P2-007 — Migration rollback compatibility and source integrity

`common.ApplyMigrations` now records a SHA-256 checksum for every declared migration.

For historical rows whose checksum predates START-23.12, the checksum is backfilled once from the currently declared source. After that, source drift causes startup failure.

New production migrations are also validated as **expand-only** by default. Destructive schema patterns such as:

- DROP TABLE / DROP SCHEMA / DROP COLUMN;
- TRUNCATE;
- column/table rename;
- ALTER COLUMN;

are rejected unless a migration explicitly opts into destructive schema handling.

The production rollback strategy is therefore:

`bad application release -> roll code back -> retain additive schema`

rather than executing unsafe automatic DOWN migrations that could destroy newly written data.

## 2.2 reload/deep-link closure

Admin deep links already rebuild authoritative Partner data from the backend.

Partner Portal navigation now persists the active view in the URL, for example:

- `/partner/app/modules`
- `/partner/app/billing`
- `/partner/app/users`

and reconstructs the selected navigation item after reload.

## Mandatory acceptance gates

Source gate:

`python3 scripts/audit_start_23_12_phase2.py`

Runtime gates:

- `sh scripts/smoke_start_23_12_phase2.sh http://127.0.0.1:8080`
- `sh scripts/smoke_start_23_12_phase2_recovery.sh http://127.0.0.1:8080`
- `sh scripts/smoke_start_23_12_phase2_runtime.sh http://127.0.0.1:18081`

Inherited Go, Flutter and Compose tests remain mandatory.

## Phase 2 acceptance rule

Phase 2 can close only if:

1. source audit is green;
2. runtime smoke is green;
3. Go + Flutter + Compose regressions remain green;
4. audit outbox recovery is proven against a real container restart;
5. durable onboarding replay returns the same Partner;
6. provisioning rows contain the complete canonical step set;
7. deployment intent replay does not create a second provider trigger;
8. migration checksum and expand-only tests pass.

Phase 3 remains closed until these conditions are met.
