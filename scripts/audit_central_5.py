#!/usr/bin/env python3
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

billing = read("services/cmd/billing/plans.go")
central5_billing = read("services/cmd/billing/central5.go")
billing_main = read("services/cmd/billing/main.go")
catalog = read("services/cmd/catalog/plans.go")
catalog_main = read("services/cmd/catalog/main.go")
central5_catalog = read("services/cmd/catalog/central5.go")
gateway = read("services/cmd/gateway/partner_portal.go")
ui = read("frontend/lib/module_control_plane.dart")
main_ui = read("frontend/lib/main.dart")
localization = read("frontend/lib/localization.dart")
partner_ui = read("frontend/lib/partner_portal.dart")
openapi = read("docs/openapi.yaml")
smoke_23112 = read("scripts/smoke_start_23_11_2.sh")
smoke_23113 = read("scripts/smoke_start_23_11_3.sh")
smoke_2312p1 = read("scripts/smoke_start_23_12_phase1.sh")

for token in [
    'case "STARTER":\n\t\treturn 10,true',
    'case "BUSINESS":\n\t\treturn 20,true',
    'case "FLEX":\n\t\treturn 0,true',
    'Premium Unlimited',
    'selectionModeUnlimited',
    '"entitlement_mode":p.SelectionMode',
    'p.SelectionMode==selectionModeUnlimited',
]:
    require(token in billing, f"billing package contract missing: {token}")

require(re.search(r"Version:\s+17\b", central5_billing) is not None, "Central-5 billing migration version 17 missing")

for token in [
    'central-5-packages-pricing-vat-unlimited',
    "display_name='Starter',monthly_price=990",
    "display_name='Business',monthly_price=1490",
    "display_name='Premium',monthly_price=2490",
    "selection_mode='UNLIMITED'",
    'vat_rate_percent',
    'tax_amount',
    'net_total',
    'applyBillingTax',
    'NET_PLUS_TAX',
    'guard_plan_invoice_mutation',
]:
    require(token in central5_billing, f"Central-5 billing migration/contract missing: {token}")

for token in [
    'central5BillingMigration()',
    '"vat_rate_percent"',
    '"vat_jurisdiction"',
    '"tax_label"',
    '"gross_total"',
]:
    require(token in billing_main, f"billing runtime VAT contract missing: {token}")

require(re.search(r"Version:\s+11\b", central5_catalog) is not None, "Central-5 catalog migration version 11 missing")

for token in [
    'partner_plan_entitlement_policies',
    "entitlement_mode IN ('FIXED','UNLIMITED')",
    'ensureDynamicPlanEntitlements',
    "publication_status='PUBLISHED'",
    "implementation_state='READY'",
    "availability='ACTIVE'",
]:
    require(token in central5_catalog, f"Central-5 catalog policy missing: {token}")

for token in [
    "pm.entitlement_source,pm.plan_key",
    '"entitlement_source":entitlementSource,"plan_key":planKey',
]:
    require(token in catalog_main, f"partner-module package provenance missing: {token}")

for token in [
    'EntitlementMode string',
    'catalogEntitlementModeUnlimited',
    'Could not resolve Unlimited plan modules',
    '"entitlement_mode":entitlementMode',
]:
    require(token in catalog, f"Catalog entitlement sync missing: {token}")

for token in [
    'mode=="SELECTABLE" || mode=="UNLIMITED"',
    'Package module configuration could not be updated',
    'Modules are controlled by your subscription package entitlement',
]:
    require(token in gateway, f"Partner Portal Premium integration missing: {token}")

for token in [
    "title: 'Packages'",
    "Starter: USD 990/month + VAT",
    "Business: USD 1,490/month + VAT",
    "Premium: USD 2,490/month + VAT",
    "Premium grows automatically",
    "all future modules",
    "Edit net price",
    "active_partner_count",
]:
    require(token in ui, f"Packages UI contract missing: {token}")

for token in [
    "VAT rate %",
    "VAT jurisdiction",
    "Tax label",
    "vat_rate_percent",
    "Package price basis",
    "Net + configured VAT",
]:
    require(token in main_ui, f"Admin VAT UI missing: {token}")

for token in [
    "'Packages':",
    "'Premium grows automatically':",
    "'VAT rate %':",
    "'Net + configured VAT':",
]:
    require(token in localization, f"Hungarian localization missing: {token}")

for token in [
    "Starter USD 990",
    "Business USD 1,490",
    "Premium USD 2,490",
    "stable FLEX API key",
    "UNLIMITED entitlement",
    "vat_rate_percent",
    "NET/tax/gross",
]:
    require(token in openapi, f"OpenAPI Central-5 contract missing: {token}")

for forbidden in [
    "Flex gives the customer up to 15 selectable modules",
    "Choose up to 15 modules",
    "Starter: USD 500/month",
    "Business: USD 1,500/month",
    "Flex: USD 2,500/month",
    "Starter is fixed at 3 modules",
    "Flex supports up to 15",
]:
    combined = ui + "\n" + main_ui + "\n" + partner_ui + "\n" + openapi
    require(forbidden not in combined, f"retired package contract survived: {forbidden}")

require("==3,by[\"STARTER\"]" not in smoke_2312p1, "Phase 1 smoke still expects Starter=3")
require("==10,by[\"BUSINESS\"]" not in smoke_2312p1, "Phase 1 smoke still expects Business=10")
require('"SELECTABLE"' not in smoke_2312p1, "Phase 1 smoke still expects Premium/FLEX SELECTABLE")
require("monthly_price\"]==500" not in smoke_23112, "recurring billing smoke still expects Starter USD 500")
require("monthly_price\"]==1500" not in smoke_23112, "recurring billing smoke still expects Business USD 1500")
require("annual_price\"]==22500" not in smoke_23112, "recurring billing smoke still expects old Flex annual price")
require("len(d[\"active_module_keys\"])==10" not in smoke_23113, "Marketplace smoke still expects Business=10")

print("Central-5 Packages/pricing/VAT/Unlimited acceptance: PASS")
