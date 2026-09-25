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
marketplace = read("services/cmd/catalog/marketplace.go")
catalog_tests = read("services/cmd/catalog/main_test.go")
central4 = read("services/cmd/catalog/central4.go")
gateway = read("services/cmd/gateway/partner_user_modules.go")
ui = read("frontend/lib/module_control_plane.dart")
localization = read("frontend/lib/localization.dart")

seed_match = re.search(r"var seedModules = \[\]seedModule\{(.*?)\n\}", catalog, re.S)
require(seed_match is not None, "seedModules block missing")
seed_rows = re.findall(r'\{"([^"]+)",\s*"([^"]+)",\s*"([^"]+)"\}', seed_match.group(1))
require(len(seed_rows) >= 1, "module catalog must contain at least one module")
keys = [row[0] for row in seed_rows]
require(len(set(keys)) == len(keys), "seed module keys must be unique")
require({"needs_assessment", "two_factor_authentication"} <= set(keys),
        "Central-4 planned modules are missing")

group_match = re.search(r"var seedGroups = \[\]seedGroup\{(.*?)\n\}", catalog, re.S)
require(group_match is not None, "seedGroups block missing")
groups = re.findall(r'\{"([^"]+)",\s*"([^"]+)",\s*(\d+)\}', group_match.group(1))
require(len(groups) >= 1, "module topic registry must contain at least one group")
group_keys = {row[0] for row in groups}
required_groups = {"finance_invoicing", "client_operations", "marketing", "website_events", "security_system"}
require(required_groups <= group_keys, f"required Central-4 topic missing: {sorted(required_groups-group_keys)}")
require(all(row[2] in group_keys for row in seed_rows),
        "every seed module must reference an existing topic")

for token in [
    '"needs_assessment": "Igényfelmérő"',
    '"two_factor_authentication": "Kétfaktoros azonosítás"',
    '"needs_assessment": true',
    '"two_factor_authentication": true',
    "central4CatalogMigration()",
]:
    require(token in catalog, f"catalog Central-4 contract missing: {token}")

for token in [
    "Version: 10",
    "catalog.module_usage_events",
    "module_usage_events_module_time_idx",
    "module_usage_events_partner_module_time_idx",
]:
    require(token in central4, f"Central-4 migration missing: {token}")

summary_match = re.search(r"var marketplaceSummaries = map\[string\]marketplaceSummary\{(.*?)\n\}", marketplace, re.S)
require(summary_match is not None, "marketplaceSummaries block missing")
summary_keys = set(re.findall(r'^\s*"([^"]+)":', summary_match.group(1), re.M))
require(set(keys) <= summary_keys, f"marketplace summaries missing: {sorted(set(keys)-summary_keys)}")
require("if len(seedModules) < 1" in marketplace,
        "Marketplace must enforce non-empty catalog rather than an exact module count")

for forbidden in [
    "expected 40",
    "len(seedModules) != 40",
    "len(marketplaceSummaries) != 40",
    "len(canonical)==40",
    "len(system)==40",
    "len(legacy)==38",
]:
    require(forbidden not in catalog_tests + marketplace,
            f"fixed catalog cardinality leaked into source/tests: {forbidden}")

for smoke_path in [
    "scripts/smoke_start_23_11_1.sh",
    "scripts/smoke_start_23_11_2.sh",
    "scripts/smoke_start_23_11_3.sh",
    "scripts/smoke_start_23_11_3j.sh",
    "scripts/smoke_start_23_11_3k.sh",
    "scripts/smoke_start_23_11_5.sh",
    "scripts/smoke_start_23_12_phase1.sh",
    "scripts/audit_start_23_11_1.py",
    "scripts/audit_start_23_11_3.py",
    "docs/START-23.11.3_ACCEPTANCE.md",
]:
    smoke = read(smoke_path)
    require('"group_key":"technical"' not in smoke,
            f"{smoke_path} still creates modules in the retired technical primary group")
    for forbidden in [
        "len(canonical)==40",
        "len(system)==40",
        "len(legacy)==38",
        "len(items)==40",
        'test "$ACTIVE_COUNT" = "38"',
        "38 canonical",
        "40 canonical",
    ]:
        require(forbidden not in smoke,
                f"{smoke_path} still hardcodes catalog cardinality via {forbidden}")

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
    "selectedGroupKey",
    "void showTopicOverview()",
    "void applyRegistryPreset(String preset)",
    "onTap: showTopicOverview",
    "applyRegistryPreset('ACTIVE')",
    "applyRegistryPreset('SOURCE_LINKED')",
    "applyRegistryPreset('RELATIONSHIPS')",
    "moveModuleToGroup",
    "Back to module topics",
    "Runtime uses · 7 days",
    "Runtime uses · 30 days",
    "Partner × Module Commercial Matrix",
]:
    require(token in ui, f"Central-4 Modules UI contract missing: {token}")

technical_literals = {
    "USD", "EUR", "GBP", "ACTIVE", "UNAVAILABLE", "DEPRECATED",
    "PUBLISHED", "UNPUBLISHED", "IN DEVELOPMENT", "LEGACY REFERENCE",
    "READY", "FEATURE", "AUTOMATION", "INTEGRATION", "REPORT",
    "REQUIRES", "INTEGRATES", "EXTENDS", "CONFLICTS", "REPLACES",
    "himatecorp2025/himate-system", "campaigns.leads | Leads generated",
}
patterns = [
    r"LText\(\s*'([^']+)'",
    r"uiLiteral\(\s*'([^']+)'",
    r"\btitle:\s*'([^']+)'",
    r"\bsubtitle:\s*'([^']+)'",
    r"\bprimaryLabel:\s*'([^']+)'",
    r"\blabel:\s*'([^']+)'",
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

print("Central-4 Modules/topic-registry/cardinality acceptance: PASS")
