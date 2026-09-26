# CENTRAL-8 — Manual QA, Analytics, Export & Loading Closure

CENTRAL-8 converts the second manual Central QA pass into executable product and regression contracts. It is not a visual-presence audit: controls are accepted only when the backing behavior works on first interaction and the UI cannot remain trapped behind an unbounded loading state.

## Authoritative scope

CENTRAL-8 covers:

1. Dashboard usability and a real latest-four-week impact window.
2. Partners KPI filtering on the first click and bulk partner export.
3. Partner Detail progressive loading, business-grouped modules and explicit empty/degraded states.
4. Packages as one canonical Package Definition surface plus a separate Package Analytics surface.
5. Package Analytics from authoritative subscription, module-usage and Partner Portal activity data.
6. Finance weekly/monthly and package-specific paid-revenue trends plus bulk finance export.
7. Impact bulk export.
8. Central route-state persistence across browser refresh.
9. Interaction hierarchy/card contrast refinements needed by manual QA.
10. Explicit prevention of endless loading loops when a secondary data source is empty or unavailable.

## Product invariants

### Functional interaction

A button, KPI, filter or export is not accepted merely because it is rendered.

- Partners KPI presets must change the visible row set on the first click.
- Package Analytics must resolve from backend data rather than duplicated frontend package definitions.
- Export actions must return downloadable CSV content from backend endpoints.
- Dashboard weekly mode must display the latest four real weekly observations, not four fabricated placeholders.
- Route state must survive browser refresh for Central workspaces.

### Loading and empty-state behavior

No Central page may use an unbounded spinner as a substitute for an empty result.

- Primary page data is loaded independently from supplementary/enrichment data.
- Supplementary Partner Detail requests have bounded timeouts.
- The primary Partner Detail record is painted before supplementary requests finish.
- Empty backend collections produce an explicit empty state.
- A failed optional secondary service may degrade that section, but must not block already-loaded primary data.
- Repeated automatic retries are not used for an empty successful response.

### Canonical package definition

The commercial baseline is authoritative in Billing:

- Starter: USD 990/month, 10 fixed modules.
- Business: USD 1,490/month, 20 fixed modules.
- Premium: USD 2,490/month, Unlimited eligible modules.
- Premium continues to use the existing FLEX plan key for backward-compatible storage/API contracts; the user-facing name is Premium.

CENTRAL-8 reapplies this state through a new Billing migration rather than editing historical migrations, so existing deployed databases are corrected as well as new databases.

### Package Analytics provenance

Package Analytics is built from real system data:

- package/subscription ownership: `billing.partner_plan_subscriptions`;
- partner master data: `partners.partners`;
- commercial/onboarding state: Billing tables;
- module usage frequency: `catalog.module_usage_events`;
- Partner Portal active-time estimate: `identity.partner_portal_activity_buckets`.

Portal active time is defined as five-minute buckets containing at least one authenticated Partner Portal request. Before activity is recorded, the UI must show an explicit no-activity state instead of inventing a duration.

### Export contracts

Bulk CSV endpoints:

- `GET /api/v1/partners/export.csv`
- `GET /api/v1/billing/packages/export.csv`
- `GET /api/v1/billing/finance/export.csv`
- `GET /api/v1/impact/export.csv`

Exports are authenticated through the existing Central gateway, use `text/csv`, and are generated from backend records rather than DOM scraping.

## Static acceptance

`scripts/audit_central_8.py` must aggregate and report all CENTRAL-8 contract drift in one run. It verifies, at minimum:

- CENTRAL-8 migrations and endpoints are registered;
- canonical package price/module rules are present;
- Package Analytics reads authoritative usage/activity sources;
- Partner Portal activity is actually recorded;
- all four CSV endpoints exist and are exposed in the Central UI where required;
- Partner Detail primary/supplementary loading is separated and bounded;
- the first-click KPI behavior and latest-four-week behavior have Flutter tests;
- route-state persistence is present;
- CENTRAL-8 static/runtime gates are wired after CENTRAL-7 in CI.

## Runtime acceptance

`scripts/smoke_central_8.sh` runs after CENTRAL-7 in the same Compose topology and proves behavior, not control existence:

1. canonical Starter/Business/Premium package values are returned by Billing;
2. Package Analytics returns subscription-backed package/partner data;
3. a real Partner Portal login/request records activity telemetry that becomes visible to Package Analytics;
4. Finance exposes four-week and twelve-month package-aware paid-revenue series;
5. Partners CSV export is downloadable and contains real partner rows;
6. Package Analytics CSV export is downloadable and contains package/subscription rows;
7. Finance CSV export returns the invoice ledger contract;
8. Impact CSV export returns the impact-record contract even when the dataset is empty.

## CI closure rule

CENTRAL-8 is accepted only when the same branch HEAD has:

- Go: SUCCESS
- Flutter: SUCCESS
- Compose: SUCCESS
- CENTRAL-8 static acceptance: SUCCESS
- CENTRAL-8 runtime acceptance: SUCCESS

The Flutter job must include the CENTRAL-8 first-click and four-week tests, and the Compose job must run CENTRAL-8 after CENTRAL-7.
