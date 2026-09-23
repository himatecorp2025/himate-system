# START-23.11.3b Acceptance — Persistent Manual QA Partner

## Purpose

A durable Partner Portal test tenant is required for manual validation of the deployed HIMATE control plane and Partner Portal through START-23.12.

## Persistent fixture

Partner:
- ID: `ptr_himate_test_001`
- display name: `HIMATE TEST PARTNER`
- lifecycle: `LIVE`
- reference partner: false
- intended use: manual QA only

Partner Portal identity:
- ID: `pusr_himate_test_001`
- email: `test.partner@himate.test`
- role: `owner`
- active: true

The raw login password is intentionally not committed to Git. Only its PBKDF2-SHA256 hash is persisted by the migration.

## Lifecycle rule

This fixture is persistent. Blueprint syncs, rebuilds and ordinary deployments must not delete it.

It remains in the system until the HIMATE system owner explicitly requests removal. Removal must be implemented as a deliberate follow-up migration that deletes the portal identity and partner fixture; it must not happen as incidental cleanup.

## Acceptance

- partner migration creates the fixed test tenant exactly once;
- identity migration creates the fixed Partner Portal owner exactly once;
- admin API can read both records;
- partner reads tolerate a NULL category_id instead of misreporting an existing partner as 404;
- no CI cleanup deletes either persistent fixture;
- the fixture is LIVE so Partner Portal authentication is allowed;
- release contract is `0.8.19-start-23.11.3b`.

Automated evidence:
- `python3 scripts/audit_start_23_11_3b.py`
- `sh scripts/smoke_start_23_11_3b.sh http://127.0.0.1:8080`
