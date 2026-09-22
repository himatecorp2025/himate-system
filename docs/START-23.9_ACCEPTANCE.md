# START-23.9 Acceptance — Real Dashboard, Analytics & Global Search

## Purpose

START-23.9 removes the final Dashboard mocks/placeholders identified by START-23.1 and replaces them with authoritative service-owned read models. It also activates permission-scoped global search without introducing a shadow data store or weakening backend authorization.

## Scope

START-23.9 closes exactly these functional-matrix contracts:

- `DASH-REVENUE`
- `DASH-PEOPLE`
- `DASH-CHART`
- `DASH-ACTIVITY`
- `DASH-SEARCH`

No START-23.10+ product scope is included.

## Revenue YTD contract

Billing remains the financial source of truth.

`GET /internal/v1/analytics/dashboard?year=YYYY` calculates paid revenue from:

- provider-settled activation licenses in `billing.initial_licenses`;
- provider-settled recurring invoices in `billing.invoices`.

Only `PAID` records inside the selected UTC calendar year are included.

Revenue is grouped by original transaction currency. START-23.9 performs **no implicit FX conversion**. The Dashboard therefore displays one monetary total when one currency is present, and an explicit mixed-currency presentation when more than one currency is present.

## People Reached contract

People Reached has one explicit authoritative definition:

`klavierhaus.events.attendance.attendee_count`

This metric is produced by the START-22 Connector from the allowlisted `events.attendance.attendee_count` aggregate and stored by the Impact service.

`GET /internal/v1/impact/dashboard?year=YYYY` returns:

- People Reached YTD;
- the metric definition/aggregation metadata;
- a 12-month trend for the same metric.

The configured metric aggregation semantics are preserved. Missing source data resolves to zero rather than synthetic data.

## Dashboard RBAC contract

Dashboard analytics never broaden domain authorization.

- Revenue YTD is returned only when the authenticated administrator has `billing.read`.
- People Reached and the Impact trend are returned only when the administrator has `impact.read`.
- A restricted domain is represented explicitly as `authorized: false`; sensitive values are omitted rather than returned as zero.
- The shared role-neutral cache may hold full internal summary state, but response shaping is performed for the authenticated administrator before the payload leaves Gateway.

## Recent Activity contract

Dashboard Recent Activity is read from the immutable `identity.audit_events` stream.

The Gateway filters every candidate event against the authenticated administrator's current read permissions before returning it. Permission-scoped activity is not stored in the shared Dashboard cache, preventing cross-role cache leakage.

No hardcoded activity rows remain.

## Global Search contract

`GET /api/v1/search?q=...` requires an authenticated administrator with Dashboard read access and never broadens authorization.

The Gateway fans out only to domains the caller may already read. START-23.9 covers:

- partners;
- module catalog;
- contact leads;
- CMS pages;
- HIMATE administration identities;
- immutable audit records.

A domain is skipped entirely when the caller lacks its corresponding `*.read` permission.

Search uses authoritative service data directly; START-23.9 does not create an eventually-consistent search shadow database.

Queries are bounded to 2–100 characters and per-domain result counts are bounded.

## Dashboard composition

`GET /api/v1/dashboard/summary?year=YYYY` aggregates:

- Partner service portfolio totals;
- Catalog module count;
- Billing YTD analytics;
- Impact People Reached and monthly trend;
- permission-filtered audit activity.

The shared ten-second cache contains only role-neutral Dashboard data. Recent Activity is injected after permission filtering on every response. Historical-year requests bypass the shared current-year cache.

If one dependent service is unavailable, the Dashboard reports a degraded state rather than fabricating values.

## Frontend acceptance

The Flutter Dashboard must:

- show real Revenue YTD data;
- show real People Reached data;
- render the Impact chart from the API-provided 12-month series;
- render Recent Activity from the audit-backed payload;
- expose an active global-search dialog from the top bar;
- remove the START-23.1 placeholder texts and hardcoded chart/activity fixtures.

Partner search results may deep-link directly into the Partner Workspace. Other results identify their authoritative workspace without inventing unsupported deep routes.

## Required automated proof

### Static audit

`scripts/audit_start_23_9.py` verifies:

- authoritative Billing and Impact read-model routes;
- the exact People Reached metric key;
- role-safe Dashboard caching;
- permission-scoped global search;
- removal of Dashboard/search placeholders and hardcoded chart fixtures;
- five START-23.9 matrix contracts closed;
- release version and CI gates.

### Compose smoke

`scripts/smoke_start_23_9.sh` proves:

1. authenticated Dashboard read;
2. provider-backed paid Billing data is reflected in Revenue YTD;
3. authoritative attendee observations change People Reached;
4. month-specific attendee observations appear in the 12-month trend;
5. a real audited mutation appears in Recent Activity;
6. System Owner global search finds partner/module/CMS/contact records;
7. a restricted Reporting administrator can search allowed partner data;
8. the same restricted administrator cannot receive CMS/contact/administration/audit search results;
9. the Reporting administrator receives Impact analytics but no Billing analytics without `billing.read`;
10. search rejects queries shorter than two characters;
11. the five historical Dashboard placeholders are absent from executable Flutter source.

## Release contract

START-23.9 release identifier:

`0.8.13-start-23.9`

## Definition of Done

START-23.9 is complete only when:

- Go tidy/vet/unit/race/OpenAPI/build passes;
- Flutter analyze/browser tests/release build passes;
- START-23 through START-23.8 audits remain green;
- START-23.1–23.6 cross-phase audit remains green;
- START-23.9 static audit passes;
- complete historical Docker Compose regression remains green;
- START-23.9 Compose smoke passes;
- all five START-23.9 functional-matrix contracts are closed;
- the pull request is merged into `develop`;
- the merge commit's `develop` CI is green.
