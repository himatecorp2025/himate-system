# START-23.11.3c Acceptance — Production Fixture Readiness

## Why this patch exists

START-23.11.3b passed in a clean Compose database but a deployed, long-lived Render database can have a migration registry that is ahead of a later-added fixture migration. A persistent manual QA fixture must not depend only on historical migration ordering.

## Production-safe fixture contract

The HIMATE TEST PARTNER and its Partner Portal owner are now also ensured idempotently during their owning service startup.

- partners service ensures `ptr_himate_test_001` exists after category bootstrap;
- gateway ensures `pusr_himate_test_001` exists after identity schema migration;
- both use conflict-safe inserts;
- ordinary startup never deletes or duplicates the fixture;
- explicit deletion remains a separate future owner-requested operation.

## Partner Portal readiness errors

Partner authentication no longer maps every registry problem to a generic 403.

- suspended/archived partner: `403 PARTNER_ACCESS_DISABLED`;
- partner record missing: `503 PARTNER_REGISTRY_NOT_READY`;
- internal credential mismatch: `503 PARTNER_REGISTRY_AUTH_FAILED`;
- registry timeout: `503 PARTNER_REGISTRY_TIMEOUT`;
- missing host wiring: `503 PARTNER_REGISTRY_UNCONFIGURED`;
- other upstream outage: `503 PARTNER_REGISTRY_UNAVAILABLE`.

Infrastructure failures do not count as credential failures and therefore do not consume the login rate-limit budget.

## Acceptance evidence

- static audit: `python3 scripts/audit_start_23_11_3c.py`;
- production-like Compose smoke: `sh scripts/smoke_start_23_11_3c.sh http://127.0.0.1:8080`;
- smoke deliberately deletes both persistent fixture rows, restarts only `partners` and `gateway`, verifies automatic repair, then performs an actual Partner Portal login across the registry readiness gate.

Release contract: `0.8.20-start-23.11.3c`.
