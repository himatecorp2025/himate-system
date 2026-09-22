# START-23.11.2 Acceptance — Calendar-Month Billing & Full-Period Charging

## Scope

START-23.11.2 replaces activation-date-anchored 30-day recurring billing with one authoritative calendar-month model while preserving the immutable historical ledger created by earlier phases.

The commercial authority established in START-23.11.1 remains unchanged: partner-specific contract pricing is authoritative, catalog prices are reference-only, and the USD minimum monthly commitment remains 1,500 unless a later approved commercial policy changes it.

## Billing period

The recurring service period is a **calendar month**:

- period start: first calendar day of the month at UTC date precision;
- period end: first calendar day of the following month, exclusive;
- invoice date: the first day of the following month;
- invoice content: the completed previous calendar month.

`cycle_days=30` is no longer a billing authority. Compatibility fields may remain for historical APIs/data, but all START-23.11.2 calculations use calendar-month boundaries.

## Full-period / no-proration rule

There is **no proration**. When a contracted module is activated during a calendar month, that module incurs its full negotiated monthly fee for that calendar month. The first-period price is resolved from the partner contract effective on the activation date and is snapshotted immutably.

A module that renews into a later month receives a new immutable monthly price snapshot resolved at that month boundary. Later contract changes cannot mutate a prior monthly snapshot or an invoiced line.

## Cancellation

Cancellation remains Billing-owned. A cancellation request keeps the module ACTIVE/CANCEL_PENDING through the current calendar month and becomes INACTIVE at the next month boundary. Withdrawal before that boundary restores ACTIVE renewal. Maintenance state remains independent.

## Minimum monthly commitment

For every generated calendar-month invoice, base service plus module fees are compared with the partner minimum monthly commitment. If the subtotal is below the minimum, an explicit immutable `MINIMUM_COMMITMENT` adjustment raises the total to that minimum. For USD contracts, the START-23.11.1 floor of USD 1,500 remains enforced.

## Ledger migration

Historical 30-day snapshots/items/invoices are retained and marked as legacy data. START-23.11.2 introduces explicit `billing_model` metadata and creates new recurring ledger records as `CALENDAR_MONTH`. Historical commercial evidence is not rewritten.

## Automated evidence

Static: `python3 scripts/audit_start_23_11_2.py`

Containerized: `sh scripts/smoke_start_23_11_2.sh http://127.0.0.1:8080`

The smoke proves calendar boundaries, full monthly mid-month charging, no proration, month-boundary cancellation, next-month-day-1 invoicing, minimum-commitment adjustment, idempotency, immutable commercial evidence, and calendar-month renewal.

## Explicitly not in START-23.11.2

- Partner Module Marketplace/self-service UX (START-23.11.3)
- default landing module per partner (START-23.11.4)
- employee-by-module permissions (START-23.11.5)
- workflow/calendar comment notifications (START-23.11.6)
- final Partner Portal closure audit (START-23.11.7)
