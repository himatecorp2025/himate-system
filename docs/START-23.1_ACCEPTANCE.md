# START-23.1 Acceptance — Functional Contract Reset & Complete UI Inventory

## Purpose

START-23.1 reopens product acceptance after the START-23 responsive merge and replaces the previous route-reachability interpretation with a mutation-complete functional contract.

START-23.1 does **not** implement the START-23.2–23.12 business fixes. It creates the authoritative inventory and CI rules that those phases must close.

## Baseline

- repository: `himatecorp2025/himate-system`
- base: `develop`
- baseline commit: `5589d72aa9e2aa78a82c22d50d4f6178fe6a6c1d`
- next security phase: START-24 is blocked until START-23.12 passes.

## Authoritative functional rule

A visible mutation is not accepted because its route returns HTTP 200/201.

A mutation is accepted only when the following chain is proven:

1. visible UI control exists and is reachable;
2. UI input validation is explicit;
3. request reaches the intended public API route;
4. backend authorization is authoritative;
5. the correct microservice owns the business rule;
6. the mutation persists to PostgreSQL and/or durable storage;
7. a subsequent read returns the persisted result;
8. the control plane records an audit/billing/domain event where required;
9. English and Hungarian presentation is complete;
10. failure/empty/disabled states are explicit;
11. automated E2E evidence executes the mutation and validates the resulting state.

A GET-only smoke is therefore **route evidence**, not functional mutation evidence.

## Required artifacts

- `docs/START-23.1_FUNCTIONAL_MATRIX.json` — machine-readable action/automation/data-model contract.
- `docs/START-23.1_SURFACE_INVENTORY.md` — human-readable page/subpage/action inventory.
- `scripts/audit_start_23_1_contract.py` — CI audit that prevents unregistered frontend mutations and invalid acceptance claims.
- `scripts/smoke_start_23_1_mutation_canary.sh` — representative write/readback/audit canary proving the new acceptance methodology.

## Functional matrix states

- `PROD_PROVEN` — source and production behavior have direct evidence.
- `SOURCE_COMPLETE_PROD_UNVERIFIED` — backend exists, but full production mutation proof is still required.
- `PARTIAL_PRODUCT` — technical path exists but product behavior is incomplete.
- `SEMANTIC_GAP` — multiple paths or state machines can produce inconsistent business results.
- `DUPLICATE_CONTROL_PATH` — duplicated UI/business control must be consolidated.
- `PLACEHOLDER` — visible interaction has no real implementation.
- `MOCK_DATA` / `MOCK_OR_EMPTY` — UI presents hardcoded or non-authoritative data.
- `MISSING_PRODUCTION_AUTOMATION` — code path exists but production scheduling/execution is absent.
- `MISSING_BACKEND_INTEGRATION` — required external/backend integration does not exist.
- `MISSING_DATA_MODEL` — the schema cannot represent the required product behavior.

## START-23.1 acceptance requirements

### Inventory completeness

Every current top-level and partner-portal surface must be represented.

Every frontend mutation call using `POST`, `PUT`, `PATCH`, `DELETE` or multipart POST must map to at least one matrix contract.

Known visible placeholders, mock dashboard data, payment automation gaps, scheduler gaps, bilingual dynamic-model gaps and the admin/Partner Portal subscription semantic split must be explicitly represented as blockers.

### No false PASS

A contract may not claim `PROD_PROVEN` unless its `e2e_proof` names an existing automated proof or an explicitly recorded production proof.

Later START-23.x phases may only move a contract to accepted/proven state after the required mutation E2E exists.

### Representative mutation canary

START-23.1 must exercise real write/readback paths for representative domains:

- partner category;
- custom module group/module;
- administrator custom role and administrator user;
- own profile update and restoration;
- contact lead mutation with audit evidence.

The canary is not a substitute for the START-23.2–23.12 domain acceptance suites. It proves that START-23.1 no longer treats GET reachability as equivalent to functionality.

## Phase ownership after START-23.1

- START-23.2 — Partner × Module control plane and individual pricing.
- START-23.3 — authoritative 30-day subscription lifecycle and production scheduler.
- START-23.4 — real payment-provider collection, webhook and invoice settlement.
- START-23.5 — complete EN/HU dynamic data model and localization.
- START-23.6 — Administration, Identity and business CRUD closure.
- START-23.7 — Impact, Evidence and Reports closure.
- START-23.8 — full CMS/design/website editing closure.
- START-23.9 — real Dashboard, analytics and global search.
- START-23.10 — System & Operations production closure.
- START-23.11 — Partner Portal commercial parity.
- START-23.12 — complete mutation E2E and production acceptance.

## Definition of Done

START-23.1 is DONE only when:

1. the machine-readable matrix passes schema/inventory validation;
2. all discovered frontend mutation calls are represented by the matrix;
3. all known blockers are present and assigned to a later START-23.x phase;
4. the representative mutation canary is green;
5. the complete pre-existing START-01–23 regression remains green;
6. the branch is merged to `develop` with no unique work stranded outside the merge path.

START-23.1 completion does **not** mean the product is functionally complete. It means the remaining incompleteness is explicit, machine-checkable and assigned to START-23.2–23.12.
