# START-23.11.3k Acceptance — Commercial Status, Charity & Package Administration

## Domain separation

Partner operational lifecycle, commercial billing mode, and module entitlement are separate authorities.

Commercial billing modes are:

- `PAID`
- `COMPLIMENTARY`
- `CHARITY`

A zero-dollar or non-paid partner can remain operationally active and can have valid module entitlements.

## Charity

Charity is never self-approved. A partner may request review, but only a HIMATE administrator with Billing approval authority can approve Charity status.

After approval:

- billing mode becomes `CHARITY`;
- recurring charge and invoice generation are disabled;
- the partner selects the published and READY modules useful to its organization;
- Charity selection has no module-count ceiling;
- Catalog entitlements remain authoritative and are tagged with Charity source;
- approval, reviewer, reason and module selection are audit persisted.

A Charity partner does not use a paid Starter / Business / Flex subscription package while Charity approval is active.

## Complimentary

`COMPLIMENTARY` is a HIMATE-controlled non-paid commercial mode. Recurring charge and invoice generation are disabled while entitlement remains independent.

## Zero-dollar commercial terms

Activation fee, base fee and minimum monthly commitment accept zero. Historical USD 1,500 minimum-commitment validation is not a platform hard gate.

A true zero-dollar activation fee is represented as explicitly waived with a reason. A billing cycle whose actual charge is zero records a billing event but does not create an invoice or payment collection.

## Central Packages

HIMATE centrally controls the standard package catalog:

- Starter — USD 500 monthly baseline, exactly 3 HIMATE-defined modules.
- Business — USD 1,500 monthly baseline, exactly 10 HIMATE-defined modules.
- Flex — USD 2,500 monthly baseline, partner-selectable, maximum 15 modules.

The 3 / 10 / 15 limits are system invariants for the standard packages. Package prices are administrator-editable without changing those limits.

Activation/license fees remain partner-specific.

## Effective-dated package pricing

Every package-price change is appended to the central price-history ledger with effective date, actor, reason and change type.

Active subscriptions resolve the central package price at their next billing boundary. Existing immutable invoice periods are not rewritten.

On January 1 the system automatically appends a price row using the configured annual uplift, currently 5%. The uplift is calculated from the price effective immediately before January 1. Therefore, if Starter changes from USD 500 to USD 600 during the year, the following January 1 price is USD 630.

## UI

HIMATE administration includes a dedicated `Packages` workspace for package prices and fixed module definitions.

Partner Pricing exposes Billing Mode and Charity review controls.

Partner Portal exposes Charity review status, Charity-request workflow, and approved Charity module management.

## Regression guard

The Partner Portal source must remain a single valid implementation. The 3k static audit fails if key portal methods are duplicated or the currency formatter is malformed.

## Release

`0.8.28-start-23.11.3k`

## Automated evidence

- `python3 scripts/audit_start_23_11_3k.py`
- `sh scripts/smoke_start_23_11_3k.sh http://127.0.0.1:8080`
- Go vet/tests
- Flutter analyze/Chrome tests
- direct PostgreSQL proof of Charity approval, entitlement count, invoice suppression, package price history, 5% January uplift and active-subscription snapshot
