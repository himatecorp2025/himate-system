# START-23.1 Surface Inventory

Baseline: `develop@5589d72aa9e2aa78a82c22d50d4f6178fe6a6c1d`

This document is generated from the authoritative machine-readable matrix. It enumerates every current product surface and the interactive/system contracts that must be closed before START-24.

## Inventory summary

- surfaces: **53**
- functional contracts: **87**
- mutation contracts: **74**
- explicit blockers/gaps: **23**

## Status interpretation

A source-complete route is not a product PASS. `SOURCE_COMPLETE_PROD_UNVERIFIED` means implementation exists but the full UI → API → backend → persistence → readback → audit → EN/HU → failure-state → E2E chain is still not accepted.

## 1. Admin authentication

Login, logout, remember-me, account redirect

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `AUTH-LOGIN` | Admin login | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `AUTH-LOGOUT` | Admin logout | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `AUTH-FORGOT` | Forgot password | placeholder | MISSING | `PLACEHOLDER` | START-23.6 |
| `AUTH-SSO` | SSO sign-in | placeholder | MISSING | `PLACEHOLDER` | START-23.6 |

## 2. Administration shell

Top-level navigation, permissions, account menu, notification center

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 3. Dashboard

KPIs, impact trend, recent activity, global search

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `DASH-REVENUE` | Revenue YTD KPI | placeholder | MISSING_AGGREGATION | `MOCK_OR_EMPTY` | START-23.9 |
| `DASH-PEOPLE` | People Reached KPI | placeholder | MISSING_AGGREGATION | `MOCK_OR_EMPTY` | START-23.9 |
| `DASH-CHART` | Impact trend chart | placeholder | MISSING_AGGREGATION | `MOCK_DATA` | START-23.9 |
| `DASH-ACTIVITY` | Recent activity | placeholder | audit service expected | `MOCK_DATA` | START-23.9 |
| `DASH-SEARCH` | Global search | placeholder | MISSING | `PLACEHOLDER` | START-23.9 |

## 4. Partners

Search, category/lifecycle/health filters, pagination, partner open

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 5. Partner categories

Category listing and creation

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-CATEGORY-CREATE` | Add category | mutation | partners | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.5 |

## 6. New Partner wizard

Partner, commercial terms, agreement, evidence, license and provisioning capture

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-CREATE` | Create partner | mutation | partners | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `PART-WIZ-TERMS` | New Partner commercial terms | mutation | billing | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |
| `PART-WIZ-AGREEMENT` | New Partner agreement | mutation | billing | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |
| `PART-WIZ-INVOICE` | New Partner activation invoice reference | mutation | billing | `PARTIAL_PRODUCT` | START-23.7 |
| `PART-WIZ-LIFECYCLE` | Advance lifecycle | mutation | partners | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `PART-WIZ-PROVISION` | Provisioning job | mutation | provisioning | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |
| `PART-WIZ-LICENSE` | Activation license payment | mutation | billing | `PARTIAL_PRODUCT` | START-23.4 |

## 7. Partner Workspace / Profile

Company, contacts, address, brand and lifecycle

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-PROFILE-EDIT` | Edit partner profile | mutation | partners | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 8. Partner Workspace / Modules

Entitlement, visibility, per-partner price, 30-day subscription and cancellation

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-MODULE-EDIT` | Edit partner module entitlement and price | mutation | catalog | `SEMANTIC_GAP` | START-23.3 |
| `PART-MODULE-CANCEL` | Schedule module cancellation | mutation | billing | `SEMANTIC_GAP` | START-23.3 |
| `MODULE-CANCELLATION-CONSISTENCY` | Admin and Partner Portal use one cancellation command | automation | catalog + billing | `SEMANTIC_GAP` | START-23.3 |

## 9. Partner Workspace / Commercial

Terms, agreement, activation license, commercial status and billing events

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-TERMS-EDIT` | Edit partner terms | mutation | billing | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |
| `PART-LICENSE-EDIT` | Register initial license/payment | mutation | billing | `PARTIAL_PRODUCT` | START-23.4 |
| `AGREEMENT-EDIT` | Edit commercial agreement | mutation | billing | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |

## 10. Partner Workspace / Finance & Documents

Commercial documents and invoice history

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-DOCUMENT-REGISTER` | Register billing/commercial document | mutation | billing | `PARTIAL_PRODUCT` | START-23.7 |

## 11. Partner Workspace / Statistics

Partner-scoped impact summary

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 12. Partner Workspace / Partner Portal Access

Tenant users and roles

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-PORTALUSER-CREATE` | Create Partner Portal user | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `PART-PORTALUSER-EDIT` | Edit Partner Portal user | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 13. Partner Workspace / Connector

Credential rotation and website adapter

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-CONNECTOR-CREDENTIAL` | Issue connector credential | mutation | connector | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |
| `WEBSITE-ADAPTER` | Edit Partner Website Adapter | mutation | connector | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |

## 14. Partner Workspace / Environments

Production environment configuration

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PART-ENV-EDIT` | Edit partner environment | mutation | environments | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |
| `PART-ENV-CREATE` | Create production environment | mutation | environments | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |

## 15. Modules / Registry

Groups, modules, metadata, pricing and availability

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `MODULE-CREATE-GROUP` | Add module group | mutation | catalog | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.5 |
| `MODULE-CREATE` | Create module | mutation | catalog | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |
| `MODULE-EDIT` | Edit module | mutation | catalog | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |
| `DYNAMIC-BILINGUAL-MODEL` | Dynamic business records support EN/HU | data_model | catalog + partners + impact + identity | `MISSING_DATA_MODEL` | START-23.5 |

## 16. Modules / Relationships

Requires, conflicts, dependencies and notes

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `MODULE-REL-CREATE` | Add module relationship | mutation | catalog | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |
| `MODULE-REL-DELETE` | Delete module relationship | mutation | catalog | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |

## 17. Modules / Impact mappings

Module-to-metric mappings

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `MODULE-METRIC-MAP` | Edit module impact metric mappings | mutation | catalog | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |

## 18. Modules / Partner usage

Partner usage visibility

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `MODULE-COMMERCIAL-MATRIX-EDIT` | Edit partner-specific recurring price, activation fee, visibility and base-package inclusion from the commercial matrix | mutation | catalog + billing read model | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.2 |

## 19. Licensing & Finance

Issuer/bank profile and commercial overview

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `BILLING-PROFILE` | Edit HIMATE billing profile | mutation | billing | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 20. Impact & Reports / Metrics

Definitions, values, baselines and summaries

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `IMPACT-DEFINITION` | Create metric definition | mutation | impact | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |
| `IMPACT-VALUE` | Record impact value | mutation | impact | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |
| `IMPACT-BASELINE` | Set impact baseline | mutation | impact | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |

## 21. Impact & Reports / Evidence

Files, URLs, declarations, integrity and verification

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `EVIDENCE-METADATA` | Create URL/declaration Evidence | mutation | evidence | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |
| `EVIDENCE-UPLOAD` | Upload Evidence file | mutation | evidence + storage | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |
| `EVIDENCE-VERIFY` | Verify Evidence | mutation | evidence | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |
| `EVIDENCE-VERIFIED-VALUE` | Record VERIFIED_DOCUMENT impact value | mutation | impact | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |

## 22. Impact & Reports / Reports

Report creation, generation, PDF download and regeneration

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `REPORT-CREATE` | Generate report | mutation | reports + storage | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |
| `REPORT-REGENERATE` | Regenerate stored report snapshot | mutation | reports + storage | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.7 |

## 23. Website & Marketing / Contact Leads

Public inquiries and internal lead workflow

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `CONTACT-LEAD-EDIT` | Update contact lead | mutation | contact | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 24. Website & Marketing / CMS Pages

Page creation, draft, sections and localization variants

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `CMS-PAGE-CREATE` | Create CMS page | mutation | cms | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.8 |
| `CMS-DRAFT-SAVE` | Save CMS draft | mutation | cms | `PARTIAL_PRODUCT` | START-23.8 |
| `CMS-ARBITRARY-SECTION` | Add arbitrary page section | placeholder | cms + gateway | `PARTIAL_PRODUCT` | START-23.8 |

## 25. Website & Marketing / CMS Media

Upload and preview media

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `CMS-MEDIA-UPLOAD` | Upload CMS media | mutation | cms + storage | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.8 |

## 26. Website & Marketing / CMS Release

Preview, publish, version history, rollback and audit

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `CMS-PREVIEW` | Create CMS preview | mutation | cms + gateway | `PARTIAL_PRODUCT` | START-23.8 |
| `CMS-PUBLISH` | Publish CMS page | mutation | cms + gateway | `PARTIAL_PRODUCT` | START-23.8 |
| `CMS-ROLLBACK` | Rollback CMS version | mutation | cms | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.8 |

## 27. Website & Marketing / Design Guide

Brand tokens, logo, navigation, draft and publish

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `DESIGN-DRAFT` | Save design draft | mutation | cms | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.8 |
| `DESIGN-PUBLISH` | Publish design | mutation | cms + public site | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.8 |
| `DESIGN-PREVIEW` | Full website design preview | placeholder | PARTIAL_UI_ONLY | `PARTIAL_PRODUCT` | START-23.8 |

## 28. Website & Marketing / SEO

SEO draft, audit and publish

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `SEO-DRAFT` | Save SEO draft | mutation | cms | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.8 |
| `SEO-PUBLISH` | Publish SEO settings | mutation | cms + gateway | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.8 |

## 29. System & Operations / Health

Service and partner health snapshots

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 30. System & Operations / Connector

START-22 mapping, sync and retention state

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 31. System & Operations / Provisioning

Provisioning jobs and state

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 32. System & Operations / Domains & Deployments

DNS/TLS, environment config and provider deployment actions

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `ENV-ACTION` | Environment provider action | mutation | environments + runtime | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |
| `ENV-CREATE` | Create environment | mutation | environments | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |
| `ENV-EDIT` | Edit environment | mutation | environments | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |

## 33. System & Operations / Backups

Policies, restore points and restore verification

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `BACKUP-CREATE` | Create backup restore point | mutation | backups | `PROD_PROVEN` | START-23.10 |
| `BACKUP-RESTORE-TEST` | Run restore verification | mutation | backups | `PROD_PROVEN` | START-23.10 |
| `BACKUP-POLICY` | Edit backup policy | mutation | backups | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.10 |

## 34. Administration / Audit

Server-side filters, pagination and immutable audit stream

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 35. Administration / Roles

Custom roles and permission matrix

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `ADMIN-ROLE-CREATE` | Create custom role | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `ADMIN-ROLE-EDIT` | Edit custom role | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 36. Administration / Administrators

Create/update platform administrators

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `ADMIN-USER-CREATE` | Add administrator | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `ADMIN-USER-EDIT` | Edit administrator | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 37. Administration / HIMATE Company

Issuer/company/bank profile

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `BILLING-PROFILE-ADMIN` | Edit HIMATE company profile from Administration | mutation | billing | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 38. Account Profile

Name, email, locale, timezone and password

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PROFILE-EDIT` | Edit own profile | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `PROFILE-PASSWORD` | Change password | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 39. Notifications

List, unread filter, mark read and mark all read

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `NOTIFY-READ` | Mark notification read | mutation | notifications | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |
| `NOTIFY-READALL` | Mark all notifications read | mutation | notifications | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 40. Partner Portal / Authentication

Tenant login, logout and session

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PORTAL-LOGIN` | Partner Portal login | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.11 |
| `PORTAL-LOGOUT` | Partner Portal logout | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.11 |

## 41. Partner Portal / Overview

Company, modules, billing and impact summary

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 42. Partner Portal / Modules

Self-service activation and period-end cancellation

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PORTAL-MODULE-ACTIVATE` | Activate module | mutation | gateway + catalog + billing sync | `PARTIAL_PRODUCT` | START-23.11 |
| `PORTAL-MODULE-CANCEL` | Cancel at period end | mutation | gateway + billing | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.3 |

## 43. Partner Portal / Billing

Current total, subscriptions and invoices

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 44. Partner Portal / Company

Allowlisted company self-service

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PORTAL-COMPANY` | Edit company profile | mutation | gateway + partners | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.11 |

## 45. Partner Portal / Users

Tenant user lifecycle

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PORTAL-USER-CREATE` | Create tenant user | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.11 |
| `PORTAL-USER-EDIT` | Edit tenant user | mutation | gateway/identity | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.11 |

## 46. Public Website / CMS

SSR/hydration of published content

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 47. Public Website / Design

Published design tokens, logo and navigation

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 48. Public Website / SEO

Published metadata, JSON-LD, sitemap and robots

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 49. Public Website / Contact

Public contact form

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PUBLIC-CONTACT` | Submit public contact inquiry | mutation | contact | `SOURCE_COMPLETE_PROD_UNVERIFIED` | START-23.6 |

## 50. Automation / Billing

30-day invoice cycle and subscription expiry

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `BILLING-SCHEDULER` | Run daily subscription/invoice cycle | automation | billing + catalog | `MISSING_PRODUCTION_AUTOMATION` | START-23.3 |

## 51. Automation / Payments

Provider charge, webhook and reconciliation

| Contract | UI / behavior | Kind | Backend | State | Closure |
|---|---|---|---|---|---|
| `PAYMENT-AUTOPAY` | Automatic recurring payment collection | automation | MISSING | `MISSING_BACKEND_INTEGRATION` | START-23.4 |

## 52. Automation / Health

Background health snapshots

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## 53. Automation / Backups

Scheduled backup and mandatory restore verification

Read-only/supporting surface. No independent mutation is currently registered for this surface; read-path behavior remains covered by the broader route/regression suites.

## Mandatory closure rule

A row may move to a proven/accepted state only when its required mutation or automation proof exists. Removing a visible control is also a valid closure only when the product decision explicitly removes the capability and the frontend/backend contract is updated consistently.
