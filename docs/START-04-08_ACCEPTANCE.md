# START-01–08 Correction Acceptance

This file is the source acceptance checklist for the correction pass. It does not authorize START-09 work.

1. Klavierhaus exists as Partner #1 and LIVE reference partner.
2. Partner categories are extensible. The HIMATE Module Catalog exposes six categories: Workshop, Finance & Invoicing, Technical Operations, Marketing, Website & Events, and Communication; the 38 verified Klavierhaus reference cards retain stable module identities.
3. Lifecycle supports PROSPECT -> LICENSE_PENDING -> READY_TO_PROVISION -> PROVISIONING -> CONFIGURATION -> TESTING -> READY_FOR_LAUNCH -> LIVE plus SUSPENDED / ARCHIVED, with invalid jumps rejected by the backend and transitions historized.
4. Partner Portfolio supports server-side search/filter/pagination and page-bounded Catalog/Billing aggregation.
5. Partner Workspace exposes all 12 control cards; START-01–08 functions use stable deep links and responsive forms, while later START-block cards remain explicitly PLANNED. The approved UI design is preserved.
6. Module Catalog contains exactly 38 verified Klavierhaus reference cards on bootstrap and supports custom modules.
7. Catalog modules have independent ACTIVE / UNAVAILABLE / DEPRECATED availability. Partner modules have ACTIVE / NOT_LICENSED / MAINTENANCE state, independent visibility and independent included-in-base flag.
8. Partner module configuration and pricing changes are historized with actor, reason and effective date; future prices do not become current early.
9. Klavierhaus activation fee is waived and base service fee is USD 2,000.
10. New-partner activation/license fee is individually configurable; USD 13,000 is the minimum for a non-waived USD license.
11. A paid initial license requires amount, payment date, payment reference, verifier and registered commercial evidence with a non-empty persistent storage URL or document reference.
12. Entering PROVISIONING fails closed until Billing confirms the initial license is PAID with registered evidence, or an explicit valid waiver applies.
13. Base service fee default annual uplift is 10% on January 1 and remains admin-overridable.
14. The partner base service period and every module subscription use activation-anchored 30-day periods. Cancellation never truncates the current paid period; at its boundary the subscription becomes inactive and the Catalog entitlement becomes NOT_LICENSED without deleting partner data.
15. HIMATE billing profile is editable; commercial document metadata can be registered with a persistent evidence reference; internal invoice records remain queryable and are not mutated in place after creation.
16. Administrator login is environment-backed; public self-registration is intentionally absent.
17. Mutating admin API calls require an authenticated platform administrator and same-origin request context.
18. Internal microservices require a service credential; JSON request bodies are bounded and strictly decoded.
19. Gateway, Partner, Catalog, Billing and Contact remain separate containerized services; private services are not exposed directly to browsers.
20. Database migrations are versioned, transactional, service-scoped and safe under parallel container startup.
21. CI must pass Go vet/tests/race/build, Flutter analyze/responsive tests/release build, OpenAPI contract check and a live Docker Compose health check before this correction package is accepted.
