# CENTRAL-14 — Administration, Backup & Recovery

## Objective

CENTRAL-14 replaces the flat Administration surface with two explicit administrative scopes while preserving microservice ownership:

1. **HIMATE Administration Center**
2. **Partner Administration Center**

No Administration monolith is introduced.

## Domain ownership

- **Partners** owns partner identity and lifecycle.
- **Billing** owns invoices, company billing identity and administrative document references.
- **Identity / Gateway** owns administrators, RBAC and immutable audit events.
- **Backups** owns encrypted restore points, restore tests and production recovery jobs.
- **Storage** owns partner media namespaces.
- **Environments / Runtime** owns deployed partner releases.
- **Connector** owns captured desired connector state.
- **Gateway** composes the Administration read model with permission-aware fields.
- **Flutter** remains presentation-oriented.

## HIMATE Administration Center

The company center exposes focused workspaces for Financial Administration, Corporate Documents, Governance / Settings / Access, and System Backup & Recovery.

The HIMATE platform database is backed up under the reserved _platform recovery scope. The normal scheduler creates encrypted restore points and runs real scratch-database restore verification. Live platform replacement is intentionally maintenance-only and cannot be executed from the running control plane.

## Partner Administration Center

The partner center renders one card per authorized partner and carries a fixed tenant scope into Financial Administration, Documents, Audit & Logs, and Backup & Recovery.

Document search is server-side and partner-scoped.

## Verified production recovery

A partner production restore is accepted only when the selected restore point is READY and has a PASSED restore test, there is no other running restore, the partner is SUSPENDED (Golden Test Partners are the controlled test exception), the request includes a reason, confirmation exactly matches RESTORE <partner_id>, and the caller has backups.approve.

Before any production mutation, Backups creates a fresh encrypted safety restore point. The restore suspends partner runtimes, restores the isolated partner database, atomically restores the Storage media namespace, and restores captured partner/environment/connector configuration.

Runtime recovery is provider-aware and remains fail-closed. If hostname, runtime config and captured active release already match, Environments verifies the actual provider runtime before reusing it. If any of those differ, recovery issues a new provider deployment under a restore-job-scoped operation ID. That operation ID is included only in the recovery deployment intent key, so ordinary START-20/START-23.12 deployment idempotency remains byte-for-byte compatible while retries of the same recovery job reuse exactly one provider deployment. Recovery finalization never advances partner lifecycle: environments remain SUSPENDED for operator verification.

If a target restore fails after production mutation started, the safety restore point is applied automatically under an independent rollback operation ID. Every production restore request emits BACKUP_PRODUCTION_RESTORE_QUEUED into the immutable central audit trail.

## Acceptance

Static: python3 scripts/audit_central_14.py

Containerized runtime: sh scripts/smoke_central_14.sh http://127.0.0.1:8080

The merge gate remains the full HIMATE CI: Go, Flutter, Compose, all historical START/CENTRAL acceptance steps and CENTRAL-14 must be green.
