# START-18–19 Acceptance — Audit, governance and RBAC

## START-18 audit
- [x] Central audit event includes actor/roles.
- [x] Semantic action is recorded.
- [x] Resource and partner scope are recorded.
- [x] Request ID and correlation ID are recorded.
- [x] Success/failure and timestamp are recorded.
- [x] Sensitive request/state keys are recursively redacted.
- [x] old_state/new_state are persisted for governance mutations.
- [x] Profile/admin-user changes capture authoritative old state.
- [x] Search/filter supports actor, action, resource, partner and correlation.
- [x] RFC3339 from/to time filtering exists.
- [x] Database trigger rejects UPDATE and DELETE on audit rows.
- [x] Audit mutation protection is covered by integration smoke.

## START-19 RBAC
- [x] Platform, Operations, Finance and Reporting roles exist.
- [x] Backend is authoritative for permissions.
- [x] read/write/approve permission classes are enforced.
- [x] Approval-grade operations require approve-level access.
- [x] Flutter hides inaccessible surfaces but is not the security boundary.
- [x] User administration is additionally protected by the single system-owner gate.
- [x] Platform-owner role cannot be delegated through normal administration APIs.

## Evidence
- `services/cmd/gateway/main.go`
- `services/cmd/gateway/main_test.go`
- `scripts/smoke_start_18_19_v2.sh`
- `scripts/smoke_profile_identity.sh`

START-18/19 are complete only when the full Go + Compose governance smoke passes.
