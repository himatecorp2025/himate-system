# START-23.11.2 Acceptance — Subscription Plans & Recurring Billing

## Scope

START-23.11.2 makes the subscription plan — not individual module price arithmetic — the recurring billing authority for the pilot product.

The module registry, partner-specific module-price history and commercial metadata delivered before this phase remain retained for future add-on/custom pricing. They do not determine recurring charges for partners on a managed subscription plan.

## Standard plans

| Plan | Monthly | Annual list price | Annual charged price | Annual saving | Modules | Selection |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Starter | USD 500 | USD 6,000 | USD 6,000 | USD 0 | 3 | fixed by HIMATE |
| Business | USD 1,500 | USD 18,000 | USD 16,500 | USD 1,500 | 10 | fixed by HIMATE |
| Flex | USD 2,500 | USD 30,000 | USD 22,500 | USD 7,500 | 15 | partner-selected |

The annual UI shows the full list price and the discounted annual price as monetary amounts. Business displays USD 18,000 → USD 16,500. Flex displays USD 30,000 → USD 22,500.

CUSTOM is a non-public plan type for individually negotiated partners such as the reference Klavierhaus account. Its recurring price and entitlements are administrator-controlled.

## Ownership

Billing owns plan definitions and prices, monthly/annual frequency, partner plan state, upgrades/downgrades, Flex selections and recurring invoice/payment authority.

Catalog owns canonical modules, publication/readiness and actual partner entitlement state. Billing synchronizes plan-derived entitlements to Catalog.

Starter and Business require exactly their configured module capacity before customer selection: Starter exactly 3 PUBLISHED + READY modules and Business exactly 10. Flex has no HIMATE-fixed module set and allows at most 15 partner-selected PUBLISHED + READY modules.

## Activation gate

A partner can view plans before activation, but cannot activate a subscription plan until the activation/license fee is provider-verified PAID or explicitly WAIVED by authorized HIMATE administration.

Activation/license fee collection remains separate from recurring plan billing.

## Monthly billing

A newly activated monthly plan becomes available immediately after the activation gate. Its first normal recurring plan charge occurs on the next calendar-month day 1. Thereafter the plan is charged on every calendar-month day 1 for the new monthly service period.

There is no individual module recurring charge for a plan-managed partner.

## Annual billing

Annual billing is prepaid. On annual-plan activation Billing creates the discounted annual charge immediately: Starter USD 6,000; Business USD 16,500; Flex USD 22,500. The next annual recurring charge is due one year later.

Invoice metadata retains both list_price and discount_amount so the ledger can reproduce the displayed annual saving.

## Upgrade

A plan upgrade within the same billing frequency is immediate and uses the complete price difference with no proration. Monthly examples: Starter → Business USD 1,000; Business → Flex USD 1,000; Starter → Flex USD 2,000.

The upgraded entitlement set is available immediately. Annual upgrades use annual charged prices and retain the original annual renewal date.

## Downgrade

There is no refund. A monthly downgrade keeps the current plan through the current period and becomes effective on the next calendar-month day 1. An annual downgrade becomes effective at annual renewal.

## Flex module changes

A normal Flex module-set change becomes effective on the next calendar-month boundary, independent of payment frequency. An immediate upgrade into Flex accepts the initial Flex selection and grants it immediately.

## Fixed-plan package changes

HIMATE administrators configure Starter and Business from Modules → Subscription Plans. The first complete fixed package can take effect immediately. Later changes default to the next calendar-month boundary and retain effective-dated history.

## Payment collection

Plan invoice generation is idempotent. Billing creates the immutable invoice/line first and then requests provider collection. Provider settlement/webhook verification remains authoritative for PAID status. Failed provider initiation remains collection-pending and retryable.

## Legacy/custom commercial data

START-23.11.1 partner-specific module prices, activation fees, quote references, price history and immutable legacy module-period evidence remain stored. They are not recurring-invoice authority for Starter/Business/Flex pilot partners.

## Automated evidence

Static audit: python3 scripts/audit_start_23_11_2.py

Containerized acceptance: sh scripts/smoke_start_23_11_2.sh http://127.0.0.1:8080

The suite must prove exact plan amounts, annual list/saving amounts, fixed module limits, Flex selection, activation gate, monthly day-1 charging, immediate annual prepay, immediate upgrade difference, scheduled downgrade, scheduled Flex set changes, Catalog entitlement sync, PLAN-only recurring invoice items, idempotency and CUSTOM support.

## Out of scope

- post-pilot per-module add-on charging beyond the managed plan price;
- additional plan tiers beyond Starter/Business/Flex/Custom;
- percentage-based promotional campaigns;
- coupons, trials or seat-based billing;
- final live-provider production proof reserved for START-23.12.
