# ADR-0006 — Durable event-driven module automation

- Status: Accepted
- Date: 2026-09-24

## Context

HIMATE is a multi-tenant platform whose partner organizations will run different combinations of business modules. Future Client Piano, Workshop Workflow, Scheduler, Notifications and Finance functionality must automate work across domain boundaries while minimizing manual administration.

Direct synchronous HTTP chains are insufficient for this requirement. They create partial-failure windows, make process restarts unsafe, tightly couple module deployment order and encourage tenant-specific business rules to leak into the platform control plane.

The existing Phase 2 onboarding saga and audit outbox demonstrated the required durability pattern but were specialized to those workflows.

## Decision

HIMATE adopts a durable event-driven automation backbone.

### Service identity

Automation producers and consumers use a service-specific HMAC identity in addition to the existing private internal credential. Signatures bind timestamp, service ID, HTTP method, path and request-body SHA-256.

### Event authority

The Automation service owns the durable event and delivery log. It does not own business state.

Business modules remain authoritative for their own state and permissions.

### Producer transaction invariant

Business modules use a local transactional outbox. A business-state mutation and its automation intent are committed in the same database transaction. A relay publishes the outbox record idempotently to the Automation service.

### Consumer invariant

Each event/consumer pair has one durable delivery record. Consumers claim records using database leases and `FOR UPDATE SKIP LOCKED`, acknowledge success explicitly and receive bounded retries before dead-letter.

### Scheduling

Future work is represented by persisted `available_at` timestamps rather than process-local timers.

### Module contracts

Catalog module manifests declare produced/consumed events, commands, scheduled actions and required permissions/modules. These declarations describe integration only; entitlement and user authorization remain separate backend authorities.

### Finance separation

HIMATE platform subscription billing remains separate from partner-customer Finance modules. Payment terms, accounting basis and allowed settlement methods are tenant policy.

Eight-day terms are not a platform rule.

## Consequences

Positive:

- modules can be added without editing a central workflow monolith;
- automation survives restarts and transient downstream outages;
- event replay is deterministic and idempotent;
- tenant/module/correlation metadata remains explicit;
- delayed reminders and overdue checks are durable;
- business modules can scale independently;
- platform subscription billing cannot accidentally become a tenant ERP ledger.

Costs:

- producers must maintain an outbox relay;
- consumers must implement idempotent handlers and explicit acknowledgement;
- service credentials must be provisioned independently;
- dead-letter monitoring and operational tooling become required production responsibilities.

## Deferred implementation

This ADR establishes the contract and runtime backbone only. It does not implement Client Piano, Workshop Workflow, Scheduler or tenant Finance business behavior. Those modules must adopt this contract when their dedicated implementation phases begin.
