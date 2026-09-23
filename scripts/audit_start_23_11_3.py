#!/usr/bin/env python3
from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]

catalog_main = (root / "services/cmd/catalog/main.go").read_text()
catalog_marketplace = (root / "services/cmd/catalog/marketplace.go").read_text()
catalog_portal = (root / "services/cmd/catalog/partner_portal.go").read_text()
gateway_portal = (root / "services/cmd/gateway/partner_portal.go").read_text()
frontend_portal = (root / "frontend/lib/partner_portal.dart").read_text()
localization = (root / "frontend/lib/localization.dart").read_text()
openapi = (root / "docs/openapi.yaml").read_text()
acceptance = (root / "docs/START-23.11.3_ACCEPTANCE.md").read_text()

def require(condition, message):
    if not condition:
        raise SystemExit("START-23.11.3 audit failed: " + message)

for token in [
    "start23113MarketplaceMigration()",
    "seedMarketplaceCatalog(ctx)",
]:
    require(token in catalog_main, "catalog main missing " + token)

for token in [
    "marketplace_visible",
    "marketplace_summary_en",
    "marketplace_summary_hu",
    "catalog_modules_marketplace_idx",
    "marketplaceExecutable",
    "marketplaceAccessState",
]:
    require(token in catalog_marketplace, "marketplace model missing " + token)

summary_entries = re.findall(r'^\s*"[^"]+"\s*:\s*\{', catalog_marketplace, flags=re.MULTILINE)
require(len(summary_entries) == 38, f"expected 38 canonical marketplace summaries, got {len(summary_entries)}")
require("len(marketplaceSummaries) != len(seedModules)" in catalog_marketplace, "canonical marketplace summary count is not fail-closed")

for token in [
    "m.marketplace_visible=TRUE OR m.publication_status='PUBLISHED'",
    '"COMING_SOON"',
    '"LOCKED"',
    '"ACTIVE"',
    '"marketplace_model": "DISCOVERY_SEPARATE_FROM_EXECUTION"',
    '"marketplace_summary"',
    '"executable"',
    '"access_state"',
]:
    require(token in catalog_portal, "Catalog Partner Portal read model missing " + token)

for token in [
    "enrichPartnerMarketplace",
    '"available_in_plans"',
    '"upgrade_plan_keys"',
    '"recommended_upgrade_plan"',
    '"in_current_plan"',
]:
    require(token in gateway_portal, "Gateway Marketplace enrichment missing " + token)

require("?locale=" in gateway_portal and "u.PreferredLocale" in gateway_portal, "Partner Portal locale is not propagated into Catalog Marketplace reads")

for token in [
    "Module Marketplace",
    "Included in your plan",
    "Explore more modules",
    "Coming soon",
    "View upgrade options",
    "marketplace_summary",
    "access_state",
]:
    require(token in frontend_portal, "Partner Portal Marketplace UI missing " + token)

for literal in [
    "Module Marketplace",
    "Catalog modules",
    "Included in your plan",
    "Explore more modules",
    "View upgrade options",
]:
    require("'" + literal.replace("'", "\\'") + "':" in localization, "Hungarian Marketplace literal missing " + literal)

require(
    "version: 0.8.17-start-23.11.3" in openapi
    or "version: 0.8.18-start-23.11.3a" in openapi
    or "version: 0.8.19-start-23.11.3b" in openapi
    or "version: 0.8.20-start-23.11.3c" in openapi
    or "version: 0.8.21-start-23.11.3d" in openapi
    or "version: 0.8.22-start-23.11.3e" in openapi
    or "version: 0.8.23-start-23.11.3f" in openapi
    or "version: 0.8.25-start-23.11.3h" in openapi
    or "version: 0.8.26-start-23.11.3i" in openapi
    or "version: 0.8.27-start-23.11.3j" in openapi
    or "version: 0.8.28-start-23.11.3k" in openapi,
    "OpenAPI contract version is not START-23.11.3 or a validated START-23.11.3 readiness patch",
)
require("/partner/api/v1/modules:" in openapi, "Partner Marketplace API is not documented")
require("Discovery visibility is not execution authority." in acceptance, "acceptance does not preserve discovery/execution boundary")
require("exactly 10 canonical modules are ACTIVE" in acceptance, "Business 10-module acceptance is missing")
require("remaining 28 canonical modules are LOCKED" in acceptance, "Business locked-module acceptance is missing")

print("HIMATE START-23.11.3 Module Marketplace audit passed")
