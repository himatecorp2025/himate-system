# CENTRAL-10 — Backend-First Read Models & Sub-800ms First Data

## Objective

Central-10 corrects the Central Admin data architecture so Go owns the authoritative
read-model work and Flutter remains a presentation client.

Acceptance targets:

- Go owns at least 95% of Central read-model responsibilities.
- Flutter performs at most 5–10% data-adjacent work and 100% of presentation.
- Each core Central screen starts from one backend read-model request.
- Backend aggregation budget is 650 ms.
- First usable HTTP data must be returned in less than 800 ms in runtime acceptance.
- Empty data is a finite empty state, never an endless spinner or fabricated zero datapoint.
- No hard-coded card/module count is introduced.
- Partner/module scale is dynamic N: the Gateway paginates partner reads and chunks Matrix source calls; per-request presentation batch sizes never cap the total dataset.

## Architecture

The read path is:

PostgreSQL / service-owned data
→ Go microservices
→ Go Gateway Central read model
→ one JSON screen model
→ Flutter rendering

Flutter may retain local presentation state such as selected tab, expanded card, hover,
animation, modal input and debounce timing. It must not join service responses, compute
authoritative KPIs, construct package pricing, group the commercial matrix, or fan out
across domain services for a screen.

## Central read models

- `GET /api/v1/central/partners`
  - partner rows, category registry, pagination, global/filter KPI counts and service enrichment.
- `GET /api/v1/central/partners/{partnerId}`
  - full Partner Workspace read model assembled in Go.
- `GET /api/v1/central/modules`
  - module registry, topics, KPI counts, backend search/filtering and grouped Partner × Module Commercial Matrix.
- `GET /api/v1/central/packages`
  - canonical package definitions, eligible modules and package analytics.
- `GET /api/v1/central/finance`
  - billing profile, ledger overview, invoices, partner labels and finance KPI model.
- `GET /api/v1/central/impact`
  - metric definitions, summary, filtered evidence and reports.

Dashboard remains `GET /api/v1/dashboard/summary`, with truthful `has_data` semantics
and Go-owned four-week elapsed Impact selection.

## Performance and resilience

- Central read models use a 650 ms request context.
- Fresh read-model cache TTL: 5 seconds.
- Stale fallback window: 45 seconds.
- Successful mutations invalidate both Gateway and browser Central read-model caches.
- Unvisited Flutter Central pages are not mounted.
- Login/session restoration prefetches only the current route's read model.
- Central browser request timeout is bounded near the 800 ms UX target.

## Commercial Matrix

Partner perspective returns partner groups ordered by partner name. Expanding a partner
shows that partner's assigned modules.

Module perspective returns module groups ordered by module name. Expanding a module shows
the partner companies using that module.

Partner, module, search and ACTIVE / NOT_LICENSED / MAINTENANCE filtering are performed by Go.

## Canonical packages

- Starter — $990 + VAT — 10 modules.
- Business — $1,490 + VAT — 20 modules.
- Premium — $2,490 + VAT — Unlimited.

The Billing database migration remains authoritative and the Central Go read model supplies
the ready-to-render display model. Flutter does not own a second package-pricing table.

## Empty/loading semantics

- Dashboard KPI values show a loading placeholder until authoritative data arrives; loading is not represented as zero.
- Impact with no recorded observations renders a no-data state.
- The chart painter does not synthesize a zero datapoint.
- Partial secondary-service failure returns a finite partial/stale read model where safe.

## Acceptance

Static:
`python3 scripts/audit_central_10.py`

Runtime:
`sh scripts/smoke_central_10.sh http://127.0.0.1:8080`

The runtime gate measures the first request for Dashboard, Partners, Partner Workspace,
Modules/Matrix, Packages, Finance and Impact and rejects responses at or above 800 ms.
