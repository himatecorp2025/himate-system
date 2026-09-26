# CENTRAL-11 — Runtime UX Closure, Golden Test Partner & CMS Preview Accuracy

## Objective

Central-11 closes the remaining runtime UX gaps observed after CENTRAL-10.1 deployment:

- first-click Central navigation must reuse warm data and avoid a second-render correction,
- loading must never masquerade as business zero,
- Golden Test Partner must provide realistic, isolated QA data across commercial, Impact,
  Evidence and Reports surfaces,
- the Golden Test Partner must have an explicit, System Owner-only factory reset that is
  technically and legally separate from normal seven-year partner archival,
- CMS/Design Guide preview must be directly actionable and simulate actual desktop, tablet
  and mobile responsive viewports,
- Hungarian localization must cover the affected CMS, Design Guide and Compliance Archive
  surfaces,
- System & Operations must expose real programmer diagnostics rather than a decorative button.

## First-click performance

Flutter login warmup uses the same exact request keys as common first interactions:

- Partners: all records, LIVE, PROSPECT and reference-only portfolio views.
- Licensing & Finance: default plus DRAFT, APPROVED, SENT and PAID invoice views.
- System & Operations: health, provisioning, environments and backup summary.

The Central shell also prebuilds the most frequently switched permission-visible workspaces
after first paint in a staggered sequence. This removes widget-mount latency without delaying
login or first content.

Partners, Licensing & Finance and Impact & Reports start with an explicit loading state so
the first Flutter frame cannot render a false zero/empty business state before the initial
read model arrives.

## Packages

Fixed Starter and Business package module selection remains authoritative:

- only modules with `publication_status=PUBLISHED` and `implementation_state=READY` are eligible,
- Starter requires exactly 10 eligible modules,
- Business requires exactly 20 eligible modules,
- insufficient inventory is rendered as an explicit configuration-availability message,
  not as a dead/inactive unexplained control.

## Golden Test Partner fixture

The fixture is allowed only when:

- the target partner already has `test_partner=true`, and
- the authenticated HIMATE actor is the System Owner.

The deterministic six-month piano-service scenario contains:

- fictional company/contact/address master data,
- full Golden Test system-module entitlements,
- commercial terms and paid activation license,
- six months of settled fictional invoice history,
- six months of piano-service operational Impact observations,
- verified fictional Partner Declaration evidence,
- a queued real `PARTNER_IMPACT` report job processed by the normal Reports worker.

All fixture content is explicitly marked QA/test data. Partner-specific views may show it,
but HIMATE platform revenue and Impact aggregates must exclude it.

## Golden Test Partner factory reset

`POST /api/v1/partners/{partnerId}/purge-test-fixture`

is a dedicated QA maintenance operation and is not the normal partner archive/delete path.

Safety contract:

- System Owner only,
- target must be `test_partner=true`,
- request body must contain an exact `confirm_partner_id`,
- purge is blocked while a Test Partner report is QUEUED/RUNNING,
- purge is blocked if a legal Compliance Archive exists,
- generated report PDF objects are removed from shared report storage,
- partner persistent-storage namespace is removed,
- Test Partner business rows are removed across domain schemas,
- partner identity is removed last,
- security audit receipts remain,
- no new seven-year Compliance Archive is created for this QA-only reset.

Normal partners continue to use the existing retention-safe archive workflow.

## CMS and Design Guide

- The in-page HIMATE website preview is clickable and opens the real draft preview.
- Preview frames simulate fixed responsive viewports:
  - Desktop: 1440 × 900
  - Tablet: 834 × 1194
  - Mobile: 390 × 844
- Device links continue to load the real draft site rather than a screenshot or placeholder.
- Hungarian localization covers the affected Design Guide, CMS and Compliance Archive
  descriptions, including dynamic labels such as `Sequence Apply Case · N`.

## System & Operations

System & Operations caches its initial multi-source future instead of rebuilding
`Future.wait()` on every Flutter rebuild.

The programmer diagnostics action reads authoritative data from:

- `/api/v1/system-health/snapshot`
- `/api/v1/audit/events?outcome=FAILED&limit=20&offset=0`

and displays degraded service state plus recent failed protected operations with request /
correlation information.

## Administration vs Compliance Archives

Administration already contains the Administrative Event Stream.

Compliance Archives remain the separate immutable seven-year legal/financial/audit record
for archived partner organizations.

A future corporate document center / partner-contract library is intentionally not conflated
with Compliance Archives and is outside this CENTRAL-11 closure.

## Acceptance

Static:

`python3 scripts/audit_central_11.py`

Runtime:

`sh scripts/smoke_central_11.sh http://127.0.0.1:8080`

The runtime acceptance must prove fixture generation, aggregate isolation, report completion
and complete Golden Test Partner factory reset on the real Compose topology.
