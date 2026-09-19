# HIMATE System

HIMATE is the central control plane for separately deployed arts-sector partner systems.

## START-04–08 completed scope
- partner registry, cards, custom categories and lifecycle
- Partner Workspace with 12 visible control cards
- canonical 38-card Klavierhaus Module Catalog + custom module creation
- ACTIVE / NOT_LICENSED / MAINTENANCE module state, visibility, base-package inclusion and partner-specific module pricing
- price history foundation
- individually configurable activation/license fee
- Klavierhaus activation fee waived; base monthly fee USD 2,000
- default 10% base-fee uplift every January 1, admin-overridable
- 30-day service cycle with one consolidated invoice issued on day 1
- HIMATE billing/issuer profile
- finance/document registry and internal invoice records
- authenticated administrator login backed by Render environment credentials

## Architecture
Go + Flutter + PostgreSQL with containerized microservices:
- public gateway + identity boundary + Flutter SPA
- private partner service
- private module catalog service
- private billing service
- monthly billing cron

Partner business databases remain separate from the HIMATE database boundary.
