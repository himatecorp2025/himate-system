# START-01–08 Correction Acceptance

This file is the source acceptance checklist for the correction pass. It does not authorize START-09 work.

1. Klavierhaus exists as Partner #1 and LIVE reference partner.
2. Partner categories are extensible. The HIMATE Module Catalog exposes six categories: Workshop, Finance & Invoicing, Technical Operations, Marketing, Website & Events, and Communication; the 38 verified Klavierhaus reference cards retain stable module identities.
3. Lifecycle supports PROSPECT -> LICENSE_PENDING -> READY_TO_PROVISION -> PROVISIONING -> CONFIGURATION -> TESTING -> READY_FOR_LAUNCH -> LIVE plus SUSPENDED / ARCHIVED, with invalid jumps rejected by the backend and transitions historized.
4. Partner Portfolio supports server-side search/filter/pagination and page-bounded Catalog/Billing aggregation.
5. Partner Workspace exposes 12 control cards, stable deep links and responsive forms while preserving the approved UI design.
6. Module Catalog contains exactly 38 verified Klavierhaus reference cards on bootstrap and supports custom modules.
7. Catalog modules have independent ACTIVE / UNAVAILABLE / DEPRECATED availability. Partner modules have ACTIVE / NOT_LICENSED / MAINTENANCE state, independent visibility and independent included-in-base flag.
8. Partner module configuration and pricing changes are historized with actor, reason and effective date; future prices do not become current early.
9. Klavierhaus activation fee is waived and base service fee is USD 2,000.
10. New-partner activation/license fee is individually configurable; USD 13,000 is the minimum for a non-waived USD license.
11. A paid initial license requires amount, payment date, payment reference, verifier and registered commercial evidence.
12. Base service fee default annual uplift is 10% on January 1 and remains admin-overridable.
13. Service cycles are anchored to the partner activation date and renew every 30 days; extra active modules consolidate into the same billing cycle.
14. HIMATE billing profile, document metadata registry and internal invoice records are editable.
15. Administrator login is environment-backed; public self-registration is intentionally absent.
16. Mutating admin API calls require an authenticated platform administrator and same-origin request context.
17. Internal microservices require a service credential; JSON request bodies are bounded and strictly decoded.
18. Gateway, Partner, Catalog, Billing and Contact remain separate containerized services; private services are not exposed directly to browsers.
19. Database migrations are versioned, transactional, service-scoped and safe under parallel container startup.
20. CI must pass Go vet/tests/race/build, Flutter analyze/responsive tests/release build, OpenAPI contract check and a live Docker Compose health check before this correction package is accepted.
