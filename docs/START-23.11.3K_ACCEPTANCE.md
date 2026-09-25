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

A Charity partner does not use a paid Starter / Business / Premium subscription package while Charity approval is active.

## Complimentary

`COMPLIMENTARY` is a HIMATE-controlled non-paid commercial mode. Recurring charge and invoice generation are disabled while entitlement remains independent.

## Zero-dollar commercial terms

Activation fee, base fee and minimum monthly commitment accept zero. Historical USD 1,500 minimum-commitment validation is not a platform hard gate.

A true zero-dollar activation fee is represented as explicitly waived with a reason. A billing cycle whose actual charge is zero records a billing event but does not create an invoice or payment collection.

## Central Packages

HIMATE centrally controls the standard package catalog:

Central-5 supersedes the original package sizes and names:
- Starter — USD 990 monthly NET baseline + configured VAT, exactly 10 HIMATE-defined modules.
- Business — USD 1,490 monthly NET baseline + configured VAT, exactly 20 HIMATE-defined modules.
- Premium — USD 2,490 monthly NET baseline + configured VAT, stable API key `FLEX`, UNLIMITED entitlement.

The 10 / 20 / Unlimited model is authoritative. Starter and Business retain fixed HIMATE-defined membership. Premium has no finite module list: every current and future eligible module is included automatically. Package net prices remain administrator-editable without changing these entitlement rules.

Activation/license fees remain partner-specific.

## Effective-dated package pricing

Every package-price change is appended to the central price-history ledger with effective date, actor, reason and change type.

Active subscriptions resolve the central package price at their next billing boundary. Existing immutable invoice periods are not rewritten.

On January 1 the system automatically appends a price row using the configured annual uplift, currently 5%. The uplift is calculated from the NET price effective immediately before January 1. The runtime acceptance changes Starter from USD 990 to USD 1,090 as an administrator-edit proof; the following January 1 NET price is therefore USD 1,144.50. VAT is then calculated from the billing-profile tax policy.

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
