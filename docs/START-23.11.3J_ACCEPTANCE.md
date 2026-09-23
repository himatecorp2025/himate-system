# START-23.11.3j Acceptance — Golden Test Partner & Partner Detail Stabilization

## Purpose

The production test tenant is a permanent Golden Test Partner used to exercise the same partner-facing flows as a real customer: module access, billing/payment scenarios, communications, notifications, permissions, integrations and future workflows.

The Golden Test Partner is identified by explicit partner master data (`test_partner=true`). HIMATE must never infer test status from display name, email address or a hard-coded partner ID.

## Golden tenant contract

- Enabling Golden Test Partner is a HIMATE administration action.
- Activation persists `test_partner=true` and moves the tenant to `LIVE`.
- Golden activation does not require a real initial-license payment or normal provisioning gate.
- The test tenant is not fake-PAID. Billing and payment states remain independently testable.
- The tenant receives all 38 canonical HIMATE modules.
- Golden entitlements are limited to canonical/system modules; temporary/custom acceptance modules are not automatically granted.
- Canonical modules remain `ACTIVE`, entitlement `ACTIVE`, visible and included for the Golden tenant.
- Partner-facing Marketplace access for the Golden tenant exposes the full canonical module set even when a module is not yet normally published for live partners.
- Normal partners retain the ordinary publication, implementation, availability, commercial and plan entitlement rules.

## Test-data isolation

Synthetic data recorded for the Golden tenant remains fully visible in the tenant's own workspace, partner-specific Impact queries and downstream test flows.

Golden tenant data is excluded by default from HIMATE platform-wide business aggregates, including:

- paid activation and recurring revenue analytics;
- global Impact / People Reached totals and trends.

This allows large or deliberately unrealistic test transactions and Impact observations without contaminating real HIMATE company reporting.

## Partner workspace loading

Core partner master data remains the blocking workspace load.

Supplementary Partner Modules, Billing, Finance, Environment, Provisioning, Impact, Connector, Portal User and Payment Profile requests load independently. Each supplementary request has a bounded timeout. A failed or slow secondary service must not keep the global partner workspace progress indicator running indefinitely.

Successfully loaded data remains usable when another supplementary source is unavailable.

## Administration UI

- Company Data contains a Golden Test Partner toggle.
- Golden activation is visibly labelled in the partner workspace and partner portfolio.
- The workspace explicitly states that test data is excluded from platform aggregates.
- Saving the Golden flag refreshes supplementary data so the 38-module entitlement set is immediately reflected.

## Deferred business rules

These accepted requirements are recorded but intentionally deferred from START-23.11.3j:

- Charity / Complimentary / Paid billing modes and HIMATE-owner charity approval;
- charity module selection with no module-count cap after approval;
- no invoice generation for true zero-dollar charity service;
- centrally managed package definitions: $500 / max 3, $1500 / max 10, $2500 / partner-selected max 15;
- annual automatic 5% package price increase and additional administrator-initiated price changes;
- account termination with a 30-day hibernation/reactivation window;
- Partner 360 growth dashboard and dedicated workspace-page navigation;
- Partner Impact / Evidence export.

Those belong to START-23.11.3k–3m and later closure work.

## Release contract

Release: `0.8.27-start-23.11.3j`.

Every deployable application microservice must report this same release version.

## Automated evidence

- Go vet and tests
- Flutter analyze and Chrome tests
- `python3 scripts/audit_start_23_11_3j.py`
- `sh scripts/smoke_start_23_11_3j.sh http://127.0.0.1:8080`
- direct PostgreSQL proof of `test_partner=true`, `LIVE` lifecycle and exactly 38 canonical Golden entitlements
- partner-scoped synthetic Impact readback
- platform aggregate proof that Golden Impact data is excluded
