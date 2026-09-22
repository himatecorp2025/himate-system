#!/usr/bin/env python3
from pathlib import Path
import re
import json

root = Path(__file__).resolve().parents[1]
catalog = (root / "services/cmd/catalog/main.go").read_text()
portal = (root / "services/cmd/catalog/partner_portal.go").read_text()
billing = (root / "services/cmd/billing/main.go").read_text()
automation = (root / "services/cmd/billing/commercial_automation.go").read_text()
frontend = (root / "frontend/lib/module_control_plane.dart").read_text()
main_ui = (root / "frontend/lib/main.dart").read_text()
localization = (root / "frontend/lib/localization.dart").read_text()
matrix = json.loads((root / "docs/START-23.1_FUNCTIONAL_MATRIX.json").read_text())

def require(condition, message):
    if not condition:
        raise SystemExit("START-23.11.1 audit failed: " + message)

seed_block = catalog.split("var seedModules = []seedModule{", 1)[1].split("\n}", 1)[0]
seed_rows = re.findall(r'\{"([^"]+)",\s*"([^"]+)",\s*"([^"]+)"\}', seed_block)
require(len(seed_rows) == 38, f"expected 38 canonical seed modules, found {len(seed_rows)}")
groups = [g for _, _, g in seed_rows]
require(groups.count("finance_invoicing") == 3, "Finance & Invoicing must contain 3 canonical modules")
require(groups.count("technical") == 16, "Technical Operations must contain 16 canonical modules")
require(groups.count("marketing") == 8, "Marketing must contain 8 canonical modules")
require(groups.count("website_events") == 11, "Website & Events must contain 11 canonical modules")
require(any(k == "workshop_workflow" and g == "technical" for k, _, g in seed_rows), "Workshop Workflow must belong to Technical Operations")

for token in [
    "publication_status TEXT NOT NULL DEFAULT 'UNPUBLISHED'",
    "implementation_state TEXT NOT NULL DEFAULT 'IN_DEVELOPMENT'",
    "legacy_reference TEXT NOT NULL DEFAULT ''",
    "entitlement_state TEXT NOT NULL DEFAULT 'INACTIVE'",
    "commercial_configured BOOLEAN NOT NULL DEFAULT FALSE",
    "contract_currency TEXT NOT NULL DEFAULT 'USD'",
    "quote_reference TEXT NOT NULL DEFAULT ''",
    '"pricing_authority":"PARTNER_CONTRACT"',
    '"publication_status":publicationStatus',
    '"entitlement_state":entitlementState',
]:
    require(token in catalog, f"missing catalog contract token: {token}")

require('publicationStatus=="PUBLISHED" && implementationState!="READY"' in catalog, "PUBLISHED must require READY implementation")
require("m.publication_status='PUBLISHED'" in portal, "Partner Portal must hide unpublished modules")
require('"MODULE_UNPUBLISHED"' in portal, "Partner activation must fail closed for unpublished modules")
require('"COMMERCIAL_TERMS_REQUIRED"' in portal, "Partner activation must require configured partner commercial terms")

for token in [
    "minimum_monthly_commitment NUMERIC(12,2) NOT NULL DEFAULT 1500",
    "pricing_model TEXT NOT NULL DEFAULT 'INDIVIDUAL_QUOTE'",
    "billing.partner_terms_history",
    "ALTER TABLE billing.partner_terms ALTER COLUMN activation_fee SET DEFAULT 0",
]:
    require(token in automation, f"missing billing commercial model token: {token}")

require("next.MinimumMonthlyCommitment < 1500" in billing, "USD 1500 minimum commitment guard is missing")
require("next.ActivationFee < 0" in billing, "activation fee must reject negative values")
require("ActivationFee < 13000" not in billing, "obsolete USD 13,000 terms activation-fee floor is still enforced")
require("next.Required < 13000" not in billing, "obsolete USD 13,000 license activation-fee floor is still enforced")
require("setCatalogEntitlementState" in billing and '"entitlement_state"' in billing, "Billing-to-Catalog entitlement synchronization is missing")
require('"quote_reference":t.QuoteReference' in billing, "partner quote reference is not exposed")
require("termsHistory" in billing and "partner_terms_history" in billing, "versioned commercial history readback is missing")
require("billing_partner_terms_history_append_only" in automation and "reject_partner_terms_history_mutation" in automation,
        "commercial terms history must be append-only at database level")
require("SELECT terms_version FROM billing.partner_terms WHERE partner_id=$1 FOR UPDATE" in billing, "commercial terms concurrent update lock is missing")
require("publication_status'] ?? 'UNPUBLISHED" in main_ui and "== 'PUBLISHED'" in main_ui,
        "new-partner module preset must be restricted to published modules")

for token in [
    "Publication status",
    "Implementation state",
    "Partner-specific contract / quote",
    "Quote / offer reference",
]:
    require(token in frontend, f"module admin UI missing: {token}")

for token in [
    "Minimum monthly commitment",
    "Individual activation fee",
    "Quote / offer reference",
]:
    require(token in main_ui, f"partner commercial UI missing: {token}")

for token in [
    "'Publication status': 'Publikációs állapot'",
    "'Implementation state': 'Implementációs állapot'",
    "'Contract currency': 'Szerződés pénzneme'",
    "'Minimum monthly commitment': 'Minimum havi vállalás'",
    "'Quote / offer reference': 'Árajánlat / ajánlat hivatkozása'",
]:
    require(token in localization, f"START-23.11.1 HU localization missing: {token}")

require(matrix.get("completed_through") == "23.11.1", "functional matrix completed_through must be 23.11.1")
surface_ids = {item["id"] for item in matrix.get("surfaces", [])}
bad_surface_refs = [(item.get("id"), item.get("surface")) for item in matrix.get("contracts", []) if item.get("surface") not in surface_ids]
require(not bad_surface_refs, f"functional matrix has invalid surface references: {bad_surface_refs}")
phase_contracts = {item["id"]: item for item in matrix.get("contracts", []) if item.get("target_phase") == "23.11.1"}
for contract_id in [
    "MODULE-REGISTRY-LIFECYCLE-23-11-1",
    "PARTNER-COMMERCIAL-TERMS-23-11-1",
    "PARTNER-MODULE-CONTRACT-PRICING-23-11-1",
]:
    require(contract_id in phase_contracts, f"functional matrix missing 23.11.1 contract: {contract_id}")
    require(phase_contracts[contract_id].get("current_state") == "MUTATION_PROVEN_PROD_UNVERIFIED", f"{contract_id} must be mutation-proven")
portal_activation = next(item for item in matrix["contracts"] if item["id"] == "PORTAL-MODULE-ACTIVATE")
require(portal_activation.get("current_state") == "PARTIAL_PRODUCT", "full Partner Portal module activation must remain open after 23.11.1")
require(portal_activation.get("target_phase") == "23.11", "Partner Portal activation target phase must remain later 23.11 scope")

print("HIMATE START-23.11.1 module registry and individual commercial model audit passed")
