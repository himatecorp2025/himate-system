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
  - materialized module registry, topics and KPI counts.
- `GET /api/v1/central/modules/commercial`
  - independently materialized Partner × Module Commercial Matrix.
- `GET /api/v1/central/packages`
  - materialized canonical package definitions.
- `GET /api/v1/central/packages/supplementary`
  - independently materialized eligible modules and package analytics.
- `GET /api/v1/central/finance`
  - materialized billing profile, ledger overview, invoices, partner labels and finance KPI model.
- `GET /api/v1/central/impact`
  - materialized metric definitions, summary, evidence and reports; filtering/pagination runs in Go against the snapshot.

Dashboard remains `GET /api/v1/dashboard/summary`, with truthful `has_data` semantics
and Go-owned four-week elapsed Impact selection.

## Performance and resilience

- Live fallback aggregation is bounded by a 650 ms backend budget; primary Central screen reads use materialized hot snapshots.
- Fresh read-model cache TTL: 30 seconds.
- Stale fallback window: 10 minutes.
- Successful mutations use route-targeted Gateway and browser invalidation; unrelated Central screens are never globally flushed.
- Flutter widgets may remain lazily mounted, but every permission-visible Central screen read model is prefetched before the first menu click.
- Modules Commercial and Packages Supplementary snapshots are also prefetched so secondary data is warm before interaction.
- Central browser request timeout is 800 ms.
- Render readiness uses dependency-aware `/api/v1/health`; degraded dependencies fail closed with HTTP 503.

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

The runtime gate measures the exact Flutter first-render URLs for Dashboard, Partners,
Modules/Commercial, Packages/Supplementary, Finance and Impact and rejects responses at
or above 800 ms. It restarts the Gateway and repeats the permission-visible Central screen
URLs from persisted materialized snapshots to prove process-cold recovery without fabricated
zero values or empty module/topic lists.
