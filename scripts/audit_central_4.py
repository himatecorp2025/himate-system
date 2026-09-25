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

catalog = read("services/cmd/catalog/main.go")
central4 = read("services/cmd/catalog/central4.go")
marketplace = read("services/cmd/catalog/marketplace.go")
gateway = read("services/cmd/gateway/partner_user_modules.go")
ui = read("frontend/lib/module_control_plane.dart")
localization = read("frontend/lib/localization.dart")

seed_match = re.search(r"var seedModules = \[\]seedModule\{(.*?)\n\}", catalog, re.S)
require(seed_match is not None, "seedModules block missing")
seed_rows = re.findall(r'\{"([^"]+)",\s*"([^"]+)",\s*"([^"]+)"\}', seed_match.group(1))
require(len(seed_rows) == 40, f"expected 40 canonical modules, got {len(seed_rows)}")
keys = {row[0] for row in seed_rows}
require({"needs_assessment", "two_factor_authentication"} <= keys, "Central-4 modules 39/40 missing")

group_match = re.search(r"var seedGroups = \[\]seedGroup\{(.*?)\n\}", catalog, re.S)
require(group_match is not None, "seedGroups block missing")
groups = re.findall(r'\{"([^"]+)",\s*"([^"]+)",\s*(\d+)\}', group_match.group(1))
group_keys = [row[0] for row in groups]
require(group_keys == [
    "finance_invoicing",
    "client_operations",
    "marketing",
    "website_events",
    "security_system",
], f"unexpected primary topic order: {group_keys}")

for token in [
    '"needs_assessment": "Igényfelmérő"',
    '"two_factor_authentication": "Kétfaktoros azonosítás"',
    '"needs_assessment": true',
    '"two_factor_authentication": true',
    "central4CatalogMigration()",
]:
    require(token in catalog, f"catalog Central-4 contract missing: {token}")

for token in [
    'Version: 10',
    'catalog.module_usage_events',
    "is_primary_navigation=FALSE WHERE group_key='technical'",
]:
    require(token in central4, f"Central-4 migration missing: {token}")

summary_match = re.search(r"var marketplaceSummaries = map\[string\]marketplaceSummary\{(.*?)\n\}", marketplace, re.S)
require(summary_match is not None, "marketplaceSummaries block missing")
summary_keys = set(re.findall(r'^\s*"([^"]+)":', summary_match.group(1), re.M))
require(keys <= summary_keys, f"marketplace summaries missing: {sorted(keys-summary_keys)}")

for token in [
    '"/internal/v1/module-usage-events"',
    '"usage_summary"',
    '"events_7d"',
    '"events_30d"',
    '"events_total"',
    '"source":"CATALOG_RUNTIME_USAGE_EVENTS"',
]:
    require(token in catalog, f"Catalog usage aggregation missing: {token}")

for token in [
    "func (a *app) recordPartnerModuleUsage",
    "status < http.StatusBadRequest",
    '"/internal/v1/module-usage-events"',
    "a.recordPartnerModuleUsage(u, key, r.Method, r.URL.Path)",
]:
    require(token in gateway, f"Gateway runtime usage contract missing: {token}")

for token in [
    "bool showCommercialMatrix = false;",
    "Widget topicGroupCard",
    "List<Map<String, dynamic>> get primaryGroups",
    "onTap: showTopicOverview",
    "applyRegistryPreset('ACTIVE')",
    "applyRegistryPreset('SOURCE_LINKED')",
    "applyRegistryPreset('RELATIONSHIPS')",
    "moveModuleToGroup",
    "Back to module topics",
    "Runtime uses · 7 days",
    "Runtime uses · 30 days",
    "Commercial Control",
]:
    require(token in ui, f"Central-4 Modules UI contract missing: {token}")

technical_literals = {
    "USD", "EUR", "GBP", "PUBLISHED", "UNPUBLISHED", "IN DEVELOPMENT",
    "LEGACY REFERENCE", "READY", "himatecorp2025/himate-system",
    "campaigns.leads | Leads generated",
}
patterns = [
    r"LText\(\s*'([^']+)'",
    r"uiLiteral\(\s*'([^']+)'",
    r"\btitle:\s*'([^']+)'",
    r"\bsubtitle:\s*'([^']+)'",
    r"\bprimaryLabel:\s*'([^']+)'",
    r"\bmessage:\s*'([^']+)'",
]
fixed = set()
for pattern in patterns:
    for value in re.findall(pattern, ui):
        if "$" not in value:
            fixed.add(value)

literal_block = localization[localization.index("static const Map<String, String> _literalHu"):]
missing = sorted(
    value for value in fixed
    if value not in technical_literals and f"'{value}':" not in literal_block
)
require(not missing, "Modules bilingual literal coverage missing: " + ", ".join(missing))

print("Central-4 Modules/topic-registry acceptance: PASS")
