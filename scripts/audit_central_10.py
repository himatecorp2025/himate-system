#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
errors = []

def read(path: str) -> str:
    p = ROOT / path
    if not p.exists():
        errors.append(f"missing file: {path}")
        return ""
    return p.read_text(encoding="utf-8")

def check(condition: bool, message: str) -> None:
    if not condition:
        errors.append(message)

frontend = read("frontend/lib/main.dart")
modules_ui = read("frontend/lib/module_control_plane.dart")
gateway = read("services/cmd/gateway/central10.go")
gateway_main = read("services/cmd/gateway/main.go")
impact = read("services/cmd/impact/main.go")
billing = read("services/cmd/billing/central8.go")
openapi = read("docs/openapi.yaml")
ci = read(".github/workflows/ci.yml")
acceptance = read("docs/CENTRAL-10_ACCEPTANCE.md")

check(bool(acceptance.strip()), "Central-10 acceptance document is missing or empty")

# Performance envelope and backend read-model contract.
for token in [
    "central10ReadBudget = 650 * time.Millisecond",
    "central10FreshTTL   = 5 * time.Second",
    "central10StaleTTL   = 45 * time.Second",
    '"architecture": "GO_BACKEND_READ_MODEL"',
    '"frontend_role": "PRESENTATION_ONLY"',
    '"target_first_usable_data_ms": 800',
]:
    check(token in gateway, f"Central-10 backend/performance contract missing: {token}")

# Every major Central read surface must have one Go read model.
for route in [
    "/api/v1/central/partners",
    "/api/v1/central/modules",
    "/api/v1/central/packages",
    "/api/v1/central/finance",
    "/api/v1/central/impact",
]:
    check(route in gateway or route in gateway_main, f"Central-10 backend read-model route missing: {route}")
    check(f"  {route}:" in openapi, f"Central-10 OpenAPI path missing: {route}")
check("/api/v1/central/partners/{partnerId}" in openapi,
      "Central-10 Partner Workspace OpenAPI path missing")

# Flutter must lazy-mount pages and prefetch only the current route read model.
for token in [
    "final Map<int, Widget> _pageCache",
    "_pageCache.putIfAbsent",
    "target = '/api/v1/central/packages'",
    "target = '/api/v1/central/finance'",
    "target = '/api/v1/central/impact'",
    "target = '/api/v1/central/modules'",
    "add('/api/v1/central')",
]:
    check(token in frontend, f"Central-10 Flutter presentation/cache contract missing: {token}")

warm_start = frontend.find("void _warmControlPlane()")
warm_end = frontend.find("Future<void> _loadPublishedBrandAssets", warm_start)
warm = frontend[warm_start:warm_end] if warm_start >= 0 and warm_end > warm_start else ""
check("paths.add('/api/v1/billing/plans')" not in warm,
      "Central-10 still contains the old eager multi-endpoint prefetch storm")
check("paths.add('/api/v1/modules')" not in warm,
      "Central-10 still eagerly prefetches raw module endpoints")

# Browser-side fan-out and obsolete read transforms are forbidden on the core paths.
for token, message in [
    ("Future<void> _loadSupplementary()", "Partner Workspace browser fan-out still exists"),
    ("Future<Map<String, dynamic>?> _safeWorkspaceGet", "Partner Workspace browser service orchestration still exists"),
    ("filteredCommercialRows", "Commercial Matrix still filters/sorts in Flutter"),
    ("commercialSubscription(", "Commercial Matrix still joins subscriptions in Flutter"),
    ("central8LatestWeeklyWindow", "Weekly-window read logic still exists in Flutter"),
    ("central9CanonicalPackage(", "Package pricing authority still exists in Flutter"),
]:
    check(token not in frontend and token not in modules_ui, message)

# One backend read model per major Flutter screen.
for token in [
    "/api/v1/central/partners",
    "/api/v1/central/packages",
    "/api/v1/central/finance",
    "/api/v1/central/impact",
]:
    check(token in frontend, f"Flutter does not consume Central-10 read model: {token}")
check("/api/v1/central/modules" in modules_ui,
      "Module Control Plane does not consume Central-10 read model")

# Truthful loading and empty-data behavior.
for token in [
    "Loading authoritative value",
    "No weekly impact data recorded yet.",
    "No monthly impact data recorded yet.",
    "if (values.isEmpty) return;",
]:
    check(token in frontend, f"Central-10 truthful loading/empty-state contract missing: {token}")
check("values.isEmpty?<double>[0]:values" not in frontend,
      "Impact chart still fabricates a zero datapoint")
check("color: hover ? brandGold.withOpacity(.08) : brandSurfaceRaised" in frontend,
      "New Partner card still uses the legacy light surface")

# Partner x Module Commercial Matrix must be backend-filtered/grouped both ways.
for token in [
    'perspective := strings.ToUpper',
    'if perspective == "MODULE"',
    '"partner_name"',
    '"modules": rows',
    '"partners": rows',
    'row["subscription"] = sub',
]:
    check(token in gateway, f"Backend Commercial Matrix contract missing: {token}")
for token in [
    "List<Map<String, dynamic>> commercialGroups",
    "Widget commercialGroupCard",
    "Modules and their partner companies",
    "Partner companies and their active services",
]:
    check(token in modules_ui, f"Grouped Commercial Matrix UI missing: {token}")

# Canonical package authority is backend-only and exact.
for token in [
    'out["monthly_price"] = 990',
    'out["display_price"] = "$990 + VAT"',
    'out["module_limit"] = 10',
    'out["monthly_price"] = 1490',
    'out["display_price"] = "$1,490 + VAT"',
    'out["module_limit"] = 20',
    'out["monthly_price"] = 2490',
    'out["display_price"] = "$2,490 + VAT"',
    'out["entitlement"] = "Unlimited"',
]:
    check(token in gateway, f"Central-10 Go package authority missing: {token}")
for token in [
    "display_name='Starter',monthly_price=990",
    "display_name='Business',monthly_price=1490",
    "display_name='Premium',monthly_price=2490",
]:
    check(token in billing, f"Canonical package DB baseline missing: {token}")

# Impact service reports whether real observations exist; gateway owns the weekly read model.
for token in [
    '"observation_count":observationCount',
    '"has_data":observationCount>0',
]:
    check(token in impact, f"Impact data-presence contract missing: {token}")
for token in [
    "central10NormalizeDashboardImpact",
    "if len(elapsed) > 4 { elapsed = elapsed[len(elapsed)-4:] }",
    'out["weekly_trend"] = elapsed',
]:
    check(token in gateway, f"Go weekly read-model contract missing: {token}")

# Responsibility score: 20 explicit read-model capabilities. 19/20 is the
# acceptance floor (95%). These are architecture responsibilities, not LOC.
responsibilities = [
    ("cache/degraded fallback", "central10StaleTTL" in gateway),
    ("bounded backend aggregation", "central10ReadBudget = 650 * time.Millisecond" in gateway),
    ("partner search/filter", 'values.Set("include_stats", "true")' in gateway),
    ("partner KPI aggregation", '"kpis": map[string]any{' in gateway),
    ("partner enrichment join", 'catalogByID :=' in gateway and 'billingByID :=' in gateway),
    ("module registry filtering", "registryPreset :=" in gateway),
    ("module KPI aggregation", '"module_registry": len(modules.Items)' in gateway),
    ("commercial search", "commercialQ :=" in gateway),
    ("commercial partner/module/status filter", "statusFilter :=" in gateway),
    ("commercial sorting", "sort.Slice(ids" in gateway and "sort.Slice(keys" in gateway),
    ("commercial grouping", '"modules": rows' in gateway and '"partners": rows' in gateway),
    ("subscription join", 'row["subscription"] = sub' in gateway),
    ("package canonicalization", "central10CanonicalPlan" in gateway),
    ("package analytics fan-out", "packageAnalytics" in gateway or '"/api/v1/billing/packages/analytics"' in gateway),
    ("finance screen aggregation", "central10Finance" in gateway and '"kpis": kpis' in gateway),
    ("impact screen aggregation", "central10Impact" in gateway),
    ("impact evidence filtering", "evidenceQuery.Set" in gateway),
    ("partner workspace aggregation", "central10PartnerWorkspace" in gateway),
    ("weekly window selection", "central10NormalizeDashboardImpact" in gateway),
    ("truthful data-presence semantics", 'out["has_data"] = false' in gateway and '"has_data":observationCount>0' in impact),
]
backend_units = sum(1 for _, ok in responsibilities if ok)
backend_share = backend_units / len(responsibilities) * 100
check(backend_share >= 95.0,
      f"Central-10 backend read-model responsibility is only {backend_share:.1f}% ({backend_units}/{len(responsibilities)})")
for name, ok in responsibilities:
    check(ok, f"Backend responsibility checkpoint failed: {name}")

# CI must enforce Central-10 after Central-9.
check("python3 scripts/audit_central_10.py" in ci, "Central-10 static gate missing from CI")
check("sh scripts/smoke_central_10.sh http://127.0.0.1:8080" in ci,
      "Central-10 runtime gate missing from CI")
check(ci.find("audit_central_9.py") < ci.find("audit_central_10.py"),
      "Central-10 static gate must run after Central-9")
check(ci.find("smoke_central_9.sh") < ci.find("smoke_central_10.sh"),
      "Central-10 runtime gate must run after Central-9")

if errors:
    print(f"FAIL: CENTRAL-10 acceptance found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    print(f"Backend responsibility score: {backend_share:.1f}% ({backend_units}/{len(responsibilities)})")
    sys.exit(1)

print(f"CENTRAL-10 backend-first architecture acceptance: PASS — backend responsibility score {backend_share:.1f}% ({backend_units}/{len(responsibilities)})")
