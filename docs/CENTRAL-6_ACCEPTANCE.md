# CENTRAL-6 — Licensing, Finance, Onboarding & Invoice Lifecycle Acceptance

## Objective

Central-6 closes the commercial activation path between Partner registration and operational Partner Portal access. Registration alone must never make a newly onboarded external Partner commercially active.

## Authoritative commercial onboarding state machine

External onboarding requests identified by a non-empty `onboarding_request_id` start in:

`REGISTERED → PENDING_REVIEW → CLASSIFIED`

Paid classifications continue through:

`INVOICE_PENDING → PAYMENT_PENDING → ADMIN_APPROVAL → ACTIVE`

Approved zero-dollar classifications (`CHARITY`, `SPONSORED`, `COMPLIMENTARY`) require an auditable support/waiver record and continue without generating a zero-dollar invoice:

`CLASSIFIED → ADMIN_APPROVAL → ACTIVE`

Partner Portal access is enabled only when the Central-6 onboarding state is `ACTIVE`. Existing HIMATE-admin-created Partner records without an onboarding request remain backward-compatible and are grandfathered as active.

## Invoice lifecycle

Commercial invoices use the authoritative workflow:

`DRAFT → APPROVED → SENT → PAID`

A non-paid invoice may also move to `CANCELLED`.

Automated recurring package and legacy billing cycles generate approval drafts. Collection and dunning are not allowed to start against draft invoices. A distributed invoice is made visible to the Partner Portal only after it reaches `SENT`; paid and cancelled distributed records remain visible for history.

## Finance controls

The Finance workspace provides:

- currency-safe invoice KPIs;
- invoice workflow queue and filters;
- manual invoice draft creation;
- approve, send, mark-paid and cancel actions;
- Partner onboarding queue and classification controls;
- 12-month paid-revenue series by currency;
- auditable finance transaction history.

Central-5 remains authoritative for package NET pricing and configured VAT. Central-6 invoices persist NET, tax and gross amounts.

## Partner delivery

Distributed invoices are available in Partner Portal Billing. A Partner can open a HIMATE-rendered PDF copy only for a Partner-visible invoice belonging to that Partner.

Sending an invoice records delivery intent and emits a Partner-scoped notification. Portal delivery is auditable independently from external email provider availability.

## Zero-dollar support

Charity, Sponsored and Complimentary access must not create a zero-dollar commercial invoice merely to advance onboarding. Instead the system stores:

- classification;
- nominal supported value;
- currency;
- approval reason;
- optional evidence reference;
- approving actor and timestamp.

This support record satisfies the commercial prerequisite for final HIMATE approval.

## Security and authority

- Onboarding mutations require `billing.approve`.
- Invoice workflow actions require `billing.approve`.
- Activation-license/provider settlement authority from START-23.4 remains intact.
- Manual Central-6 `Mark paid` applies to an invoice workflow and does not reintroduce editable provider-backed activation-license payment fields.
- Partner Portal access remains additionally blocked for suspended or archived operational Partner lifecycle states.

## Required acceptance evidence

Central-6 is not mergeable until the final PR HEAD passes all three CI jobs:

1. Go
2. Flutter
3. Compose

The Central-6-specific gates are:

- `scripts/audit_central_6.py`
- `scripts/smoke_central_6.sh`

The runtime smoke proves both a paid onboarding path and a zero-dollar Sponsored path, including pre-activation Portal denial, invoice approval/distribution/payment, final activation, Partner-visible PDF, Finance overview and persisted audit ledgers.
