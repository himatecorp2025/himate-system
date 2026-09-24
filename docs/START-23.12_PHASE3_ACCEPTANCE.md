# START-23.12 Phase 3 — Automation Architecture & Financial Integrity Acceptance

## Purpose

Phase 3 was re-scoped after reviewing the final multi-tenant operating model. HIMATE must support many partner organizations, not only Klavierhaus, and the future Client Piano, Workshop Workflow, Scheduler, Notifications and Finance modules must connect through reusable automation contracts instead of partner-specific hard-coded workflow chains.

This acceptance therefore closes the platform foundation required for later module automation while preserving the explicit Phase 2 boundary: business-module runtimes that do not exist yet are not fabricated merely to satisfy an audit.

## Scope correction

The existing `services/cmd/billing` domain is HIMATE platform subscription/commercial billing. It is not a partner ERP customer-invoicing module and must never become one.

Accordingly:

- HIMATE subscription billing remains responsible for partner plans, platform invoices, provider collection and entitlement/commercial state.
- Future tenant Finance modules will own partner-customer invoices, payment terms, settlement methods, ledger policy and accountant export.
- Future Workshop/Client Piano/Scheduler modules will own their business state and publish standard automation events.
- Cross-module orchestration uses the durable Automation service and transactional producer outbox contract.

`MODULE_RUNTIME_DEFERRED`: Client Piano, Workshop Workflow, Scheduler and tenant Finance business runtimes remain deferred until their dedicated module implementation phases. No fake runtime endpoints or placeholder business records were introduced.

## Phase 1A — Machine-to-machine Zero-Trust automation boundary

Automation calls require two independent controls:

1. the existing private internal service credential, and
2. a service-specific HMAC identity using `X-Himate-Service-ID`, timestamp and body-bound SHA-256 signature.

The Automation service resolves a distinct secret for each allowed producer/consumer identity through `HIMATE_AUTOMATION_SERVICE_KEYS_JSON`. A service can register only subscriptions for its own authenticated identity, claim only its own deliveries, and acknowledge/fail only deliveries assigned to itself.

Signed requests expire after five minutes and body/path tampering invalidates the signature.

This supplements, not replaces, tenant/module authorization inside the business module that creates the event.

## Phase 2A — Durable event-driven automation backbone

The private Automation microservice persists:

- immutable domain events;
- consumer subscriptions;
- one delivery record per event/consumer;
- delivery attempt count;
- lease state;
- retry availability;
- terminal dead-letter state;
- partner/module/correlation/causation metadata.

Event ingestion is idempotent on `(producer_service,event_key)`. Reuse of the same key with a different normalized event envelope fails closed with `EVENT_KEY_CONFLICT`.

Consumers use lease-based pull delivery with `FOR UPDATE SKIP LOCKED`, allowing horizontal workers without duplicate concurrent ownership. Failed deliveries return to PENDING after a configurable retry delay and become `DEAD_LETTER` after the configured maximum number of attempts.

`available_at` is part of the event contract, so future reminders, overdue checks and delayed workflow actions do not require sleep loops or in-process timers.

### Transactional producer outbox

`services/internal/automation` provides a reusable local outbox contract.

A producer inserts its business mutation and automation event into the same database transaction via `EnqueueTx`. The relay later claims outbox records with a lease, publishes them idempotently to the Automation service, then marks them PUBLISHED.

This creates the required crash invariant:

> business state committed => automation intent is durably committed with it.

Future partner-runtime modules using isolated databases can install the same outbox schema in their own database.

## Module automation manifest

Catalog remains the module-control-plane authority.

A module may declare an `automation` object in its existing machine-readable manifest. Contract version 1 supports:

- `produces_events`
- `consumes_events`
- `commands`
- `scheduled_actions`
- `required_permissions`
- `required_modules`

Catalog validates this contract on create/update. The automation declaration does not grant entitlement, permission or execution access; existing organization entitlement and per-user module authorization remain authoritative.

Illustrative future contracts include:

- `client_piano.saved.v1`
- `workflow.started.v1`
- `workflow.phase.completed.v1`
- `workflow.phase.delayed.v1`
- `workflow.qc.passed.v1`
- `finance.invoice.issued.v1`
- `finance.payment.settled.v1`

These names describe the intended integration contract only. Their business handlers remain deferred with the corresponding modules.

## Phase 3A — HIMATE platform billing integrity

The audit found a real crash window in plan billing: an invoice could commit before its PLAN line item, then a replay could return the existing invoice without repairing the missing item.

The corrected behavior is:

- plan invoice creation begins an explicit SQL transaction;
- existing/replayed invoice is locked `FOR UPDATE`;
- the plan line item is inserted/reconciled before commit;
- replay verifies that the stored line item still belongs to the expected invoice/partner and amount;
- `INVOICE_GENERATED` is written in the same transaction;
- a billing-event failure aborts the financial transaction;
- collection is queued only after commit.

Legacy subscription invoice assembly now also commits invoice, attached items, calculated total and billing events in one transaction before collection.

Stripe webhook cryptographic verification and payment-settlement idempotency from the previous implementation remain unchanged.

## Phase 3B — Reusable tenant Finance policy contract

`CONFIGURABLE_PAYMENT_TERMS`

Payment terms are policy, not a Klavierhaus hard-coded rule.

The reusable resolver uses this precedence:

1. invoice-specific override;
2. service/workflow default;
3. partner default;
4. platform-configured fallback.

The core accepts 0–365 days. Eight days is therefore only one possible configured default. Klavierhaus may use eight days for a class of invoices and still override individual invoices; another partner may use different defaults.

The same finance-policy contract normalizes:

- `accounting_basis = CASH | ACCRUAL`;
- partner-specific enabled payment methods.

The future tenant Finance module will persist the selected policy and snapshot the effective terms/method on each issued invoice. No tenant accounting rule is written into HIMATE platform subscription billing.

## Intended future automation chain

When the deferred modules are later implemented, they should compose through events rather than direct cross-module state mutation:

```text
Client Piano saved
  -> durable client_piano.saved event
  -> Workshop workflow created from template
  -> phase completion events
  -> Scheduler dependency/capacity recalculation
  -> workflow.qc.passed
  -> tenant Finance invoice intent
  -> terms/payment policy snapshot
  -> invoice issue/delivery
  -> scheduled reminder/overdue events
  -> provider or approved manual settlement
  -> accounting posting according to tenant basis
  -> receipt/export/notification events
```

Every arrow is required to be idempotent, tenant-scoped, correlation-aware and recoverable after process restart.

## Acceptance gates

Phase 3 source acceptance verifies:

- service-specific signed automation identity;
- immutable event log;
- idempotent event keys and conflict detection;
- durable subscription/delivery queue;
- retry, lease and dead-letter behavior;
- future `available_at` delivery;
- transactional producer outbox;
- validated module automation manifests;
- atomic platform-billing invoice assembly;
- configurable payment terms and accounting basis;
- Docker Compose and Render topology;
- mandatory CI source/runtime gates.

The runtime smoke proves signed producer/consumer publication, replay idempotency, conflicting replay rejection, tenant/module/correlation propagation, dead-letter behavior, future scheduling and tampered-signature rejection.

## Auditor conclusion

Phase 3 does not claim that deferred business modules already exist.

It closes the architecture that those modules must use and repairs the financial-integrity defect in the existing HIMATE platform billing domain. Phase 4 may begin only after the Phase 3 pull request passes the complete CI suite and is merged into `develop`.
