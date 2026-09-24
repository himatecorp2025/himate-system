#!/usr/bin/env python3
from pathlib import Path
import sys

root = Path(__file__).resolve().parents[1]
billing_modes = (root / "services/cmd/billing/commercial_modes.go").read_text(encoding="utf-8")
plans = (root / "services/cmd/billing/plans.go").read_text(encoding="utf-8")
billing = (root / "services/cmd/billing/main.go").read_text(encoding="utf-8")
catalog_plans = (root / "services/cmd/catalog/plans.go").read_text(encoding="utf-8")
gateway_portal = (root / "services/cmd/gateway/partner_portal.go").read_text(encoding="utf-8")
frontend = (root / "frontend/lib/main.dart").read_text(encoding="utf-8")
portal = (root / "frontend/lib/partner_portal.dart").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")
compose = (root / "docker-compose.yml").read_text(encoding="utf-8")

release = "0.8.30-start-23.11.5"

checks = [
    (
        "commercial mode is independent from partner lifecycle and entitlement",
        "billing_mode TEXT NOT NULL DEFAULT 'PAID'" in billing_modes
        and "CHECK(billing_mode IN ('PAID','COMPLIMENTARY','CHARITY'))" in billing_modes
        and "charity_status TEXT NOT NULL DEFAULT 'NOT_REQUESTED'" in billing_modes,
    ),
    (
        "Charity requires explicit HIMATE approval",
        "CHARITY_APPROVAL_REQUIRED" in billing_modes
        and "CHARITY billing mode requires explicit HIMATE approval" in billing_modes
        and "charity_reviewed_by" in billing_modes,
    ),
    (
        "approved Charity has unlimited direct module selection",
        "partner_charity_module_selections" in billing_modes
        and '"module_limit":nil' in billing_modes
        and "syncCharityEntitlements" in billing_modes
        and '"CHARITY"' in catalog_plans,
    ),
    (
        "zero-dollar and non-paid billing never create invoices",
        "if amount <= 0" in plans
        and "ZERO_DOLLAR_BILLING_CYCLE" in plans
        and "mode.BillingMode!=billingModePaid || nominalTotal<=0" in billing
        and "ZERO_DOLLAR_BILLING_CYCLE" in billing,
    ),
    (
        "legacy USD 1500 minimum is not a hard validation gate",
        "MINIMUM_MONTHLY_COMMITMENT" not in billing
        and "Commercial amounts cannot be negative" in billing,
    ),
    (
        "standard package limits are fixed at 3 / 10 / 15",
        'case "STARTER":\n\t\treturn 3,true' in plans
        and 'case "BUSINESS":\n\t\treturn 10,true' in plans
        and 'case "FLEX":\n\t\treturn 15,true' in plans
        and "STANDARD_PACKAGE_LIMIT" in plans,
    ),
    (
        "central package pricing is effective-dated and auditable",
        "billing.subscription_plan_price_history" in billing_modes
        and "effective_from" in billing_modes
        and "loadPlanAt" in plans
        and "change_type" in plans,
    ),
    (
        "automatic package uplift is 5 percent on January 1",
        "annual_increase_percent NUMERIC(6,2) NOT NULL DEFAULT 5" in billing_modes
        and "UPDATE billing.partner_terms SET annual_increase_percent=5" in billing_modes
        and "ensureAnnualPlanIncreases" in plans
        and "Automatic %.2f%% January 1 package increase" in plans,
    ),
    (
        "renewal resolves central package price at the billing boundary",
        "p,err:=a.loadPlanAt(ctx,s.PlanKey,start)" in plans
        and "monthly_price_snapshot=$5" in plans,
    ),
    (
        "Partner Portal exposes Charity request and module management",
        'path=="/charity"' in gateway_portal
        and 'path=="/charity/request"' in gateway_portal
        and 'path=="/charity/modules"' in gateway_portal
        and "/partner/api/v1/charity/request" in portal
        and "/partner/api/v1/charity/modules" in portal
        and "Manage Charity modules" in portal,
    ),
    (
        "HIMATE admin exposes central Packages and Charity controls",
        "class PackagesPage" in frontend
        and "Central subscription packages, prices and module entitlements." in frontend
        and "Charity review status" in frontend
        and "Complimentary" in frontend,
    ),
    (
        "Partner Portal source is single-copy and structurally guarded",
        portal.count("class _PartnerPortalShellState") == 1
        and portal.count("Future<List<String>?> chooseFlexModules") == 1
        and portal.count("Future<List<String>?> chooseCharityModules") == 1
        and portal.count("Future<void> requestCharityReview") == 1
        and portal.count("Future<void> manageCharityModules") == 1
        and portal.count("Widget overview()") == 1
        and portal.count("Widget billingPage()") == 1
        and "NumberFormat.currency(symbol: r'$', decimalDigits: 0)" in portal,
    ),
    (
        "OpenAPI publishes START-23.11.3k contract",
        "version: " + release in openapi
        and "START-01 through START-23.11.5" in openapi
        and "commercial-mode" in openapi
        and "Charity" in openapi,
    ),
    (
        "all deployable application services share the 3k release",
        render.count("value: " + release) == 19
        and compose.count("HIMATE_APP_VERSION: ${HIMATE_APP_VERSION:-" + release + "}") == 18,
    ),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for failure in failures:
        print("FAIL:", failure)
    sys.exit(1)

print("START-23.11.3k Commercial Status, Charity & Package Administration static audit: PASS")
