# START-23.3 Acceptance — Authoritative Period-End Cancellation State Machine

## Purpose

START-23.3 closes the entitlement/cancellation semantic gaps left after START-23.2.

Billing is the single authority for module subscription lifecycle. Catalog owns module assignment metadata and pricing, but an active paid-period entitlement cannot be terminated directly through Catalog administration.

## Authoritative states

Billing exposes these lifecycle states on every module subscription:

- `ACTIVE` — current paid 30-day period is active and renews;
- `CANCEL_PENDING` — current paid period remains active, renewal is disabled, cancellation becomes effective exactly at `period_end`;
- `INACTIVE` — the paid period ended and Billing completed entitlement deactivation.

The legacy `auto_renew`, `cancel_at_period_end` and `payment_status` fields remain compatible read fields, but `lifecycle_state` is authoritative for START-23.3 behavior.

## Command ownership

Admin and Partner Portal both schedule or withdraw cancellation through:

`PATCH /api/v1/billing/partners/{partnerId}/subscriptions/{moduleKey}`

Partner Portal may wrap that route, but must not implement an independent state machine.

Catalog must reject any external admin transition from a live or maintenance entitlement into `NOT_LICENSED` with `BILLING_LIFECYCLE_REQUIRED`. This includes the indirect `ACTIVE -> MAINTENANCE -> NOT_LICENSED` bypass. Billing may perform the final internal Catalog deactivation after the paid period closes.

## Period-end behavior

Scheduling cancellation must:

1. preserve the current immutable period and price snapshot;
2. preserve Catalog entitlement/access through the period end;
3. set `lifecycle_state=CANCEL_PENDING`;
4. set `auto_renew=false`;
5. store requester, reason, request time and cancellation effective date;
6. append subscription history and Billing event evidence.

Withdrawal before period end must return the subscription to `ACTIVE`, clear cancellation metadata, and restore renewal.

At or after the exact period-end boundary, the Billing cycle must:

1. close the paid period exactly once;
2. avoid creating a renewal period;
3. internally change Catalog entitlement to `NOT_LICENSED`;
4. set Billing lifecycle to `INACTIVE`;
5. remain idempotent on repeated or missed-run catch-up execution.

## Partner Portal

Partner Portal cancellation availability is determined by the Billing subscription lifecycle, not by the current Catalog status. A temporary Catalog state such as maintenance must not make an existing paid subscription impossible to cancel.

Base-package modules remain non-cancellable individually.

## Production automation

The Render Blueprint must declare the daily `himate-30day-invoice-cycle` cron using the Billing image and `/app/service --run-invoice-cycle`.

The daily cadence is intentional: each run evaluates all module period boundaries and safely catches up missed periods using immutable point-in-time snapshots.

## Required proof

`scripts/smoke_start_23_3.sh` must prove:

1. active module creates a Billing subscription;
2. direct admin `ACTIVE -> NOT_LICENSED` is rejected;
3. `ACTIVE -> MAINTENANCE -> NOT_LICENSED` cannot bypass Billing;
4. admin cancellation schedules `CANCEL_PENDING`;
5. the current entitlement is not deactivated before period end;
6. Partner Portal can withdraw the same cancellation even while Catalog is in maintenance;
7. Partner Portal can schedule it again through the same Billing authority;
8. a Billing cycle executed at the exact period end changes the lifecycle to `INACTIVE`;
9. Catalog becomes `NOT_LICENSED` only after Billing expiry;
10. no renewal period is created;
11. re-running the cycle is idempotent;
12. subscription history and Billing events retain lifecycle evidence.

## Definition of Done

START-23.3 is complete only when:

- Go tidy/vet/unit/race/build passes;
- Flutter analyze/browser tests/release build passes;
- all START-01–23.2 regression gates remain green;
- START-23.3 static audit passes;
- START-23.3 mutation/state-machine smoke passes in Docker Compose;
- Render cron validation passes;
- the feature branch is merged to `develop` with no unique commits left outside the merge path.

START-24 remains blocked until START-23.4–23.12 are complete.
