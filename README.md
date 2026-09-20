# HIMATE System

HIMATE is the central control plane for separately deployed arts-sector partner systems.

## START-01–08 corrected scope
- authenticated administrator control plane with explicit REST contract
- versioned, service-scoped PostgreSQL migrations with serialized bootstrap
- partner registry, extensible categories and backend-enforced lifecycle transitions
- paginated Partner Portfolio with bounded cross-service aggregation
- Partner Workspace with stable deep links and responsive admin forms
- canonical 38-card Klavierhaus Module Catalog + custom module creation
- catalog availability plus partner ACTIVE / NOT_LICENSED / MAINTENANCE state
- independent partner visibility, base-package inclusion and effective-dated module pricing
- auditable module/configuration and price history with actor/reason metadata
- individually configurable initial activation/license fee and auditable payment state
- Klavierhaus activation fee waived; base service fee USD 2,000
- default 10% base-fee uplift every January 1, admin-overridable
- activation-date anchored 30-day service cycles and consolidated module charges
- HIMATE billing/issuer profile, commercial evidence registry and internal invoice records
- responsive Flutter administration UI without redesigning the approved visual system

## Security and performance baseline
- HttpOnly SameSite=Strict administrator session cookie; Secure in production
- same-origin mutation protection, platform-admin mutation authorization and login throttling
- authenticated private service-to-service traffic
- bounded JSON request bodies and strict JSON decoding
- hardened response headers and no-store policy for API data
- paginated partner reads, short-lived dashboard cache and page-bounded catalog/billing aggregation
- Go race tests, Flutter responsive widget tests and full container topology health test in CI

## Architecture
Go + Flutter + PostgreSQL with containerized microservices:
- public Gateway / Identity boundary + Flutter SPA + public marketing frontend
- private Partner service
- private Module Catalog service
- private Billing service
- private Contact service
- daily billing-cycle checker that emits invoices only on each partner's 30-day boundary

Each service is built into its own non-root container. Local Docker Compose mirrors the Render service boundaries. Partner business databases remain separate from the HIMATE database boundary.
