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

## Payment collection and dunning

Plan invoice generation is idempotent. Billing creates the immutable invoice/line first and then requests provider collection. Provider settlement/webhook verification remains authoritative for PAID status. The saved provider payment method is charged automatically; the partner is not required to manually pay each monthly invoice.

Recurring monthly and annual-renewal invoices use a three-attempt dunning schedule relative to the invoice due date:
- attempt 1: due date / day 1;
- attempt 2: due date + 2 days / day 3;
- attempt 3: due date + 5 days / day 6.

Each charge uses a distinct idempotency key. After the first or second verified failure the subscription is PAST_DUE and the next retry date is recorded/notified. After the third verified failure the partner plan becomes SUSPENDED, the partner lifecycle becomes SUSPENDED and active plan entitlements are removed.

The suspension opens a 30-day cure window. A successful payment inside that window restores the prior partner lifecycle, the plan ACTIVE state and the plan-derived entitlements without deleting tenant history.

If payment is still unresolved 30 days after suspension (day 36 relative to a day-1 monthly due date), the subscription becomes CANCELLED, the partner lifecycle becomes ARCHIVED, Partner Portal login identities are deleted and current operational plan selections/entitlements are purged. Financial invoices, payment settlements, contract/commercial history and immutable audit/evidence are retained under the legal retention policy and are never deleted by the dunning purge.

The commercial contract/Terms must clearly disclose automatic recurring card collection, the retry schedule, suspension, the cure period and the operational-purge consequence before the partner authorizes recurring billing.

## Legacy/custom commercial data

START-23.11.1 partner-specific module prices, activation fees, quote references, price history and immutable legacy module-period evidence remain stored. They are not recurring-invoice authority for Starter/Business/Flex pilot partners.

## Automated evidence

Static audit: python3 scripts/audit_start_23_11_2.py

Containerized acceptance: sh scripts/smoke_start_23_11_2.sh http://127.0.0.1:8080

The suite must prove exact plan amounts, annual list/saving amounts, fixed module limits, Flex selection, activation gate, automatic monthly day-1 charging, immediate annual prepay, immediate upgrade difference, scheduled downgrade, scheduled Flex set changes, Catalog entitlement sync, PLAN-only recurring invoice items, idempotency, the day-1/day-3/day-6 retry schedule, third-failure suspension, cure-window recovery, day-36 operational purge with legal-ledger retention, and CUSTOM support.

## Out of scope

- post-pilot per-module add-on charging beyond the managed plan price;
- additional plan tiers beyond Starter/Business/Flex/Custom;
- percentage-based promotional campaigns;
- coupons, trials or seat-based billing;
- final live-provider production proof reserved for START-23.12.
