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
step3_snapshots = read("services/cmd/gateway/central_step3_snapshots.go")
step4_snapshots = read("services/cmd/gateway/central_step4_snapshots.go")
dashboard_snapshots = read("services/cmd/gateway/dashboard_snapshot.go")
render = read("render.yaml")
impact = read("services/cmd/impact/main.go")
billing = read("services/cmd/billing/central8.go")
openapi = read("docs/openapi.yaml")
ci = read(".github/workflows/ci.yml")
acceptance = read("docs/CENTRAL-10_ACCEPTANCE.md")
smoke = read("scripts/smoke_central_10.sh")

check(bool(acceptance.strip()), "Central-10 acceptance document is missing or empty")

# Performance envelope and backend read-model contract.
for token in [
    "central10ReadBudget = 650 * time.Millisecond",
    "central10FreshTTL   = 30 * time.Second",
    "central10StaleTTL   = 10 * time.Minute",
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
check("/api/v1/central/partners/{partnerId}/modules" in openapi,
      "Central-10 Partner Workspace module read-model OpenAPI path missing")

# Flutter widgets may remain lazily mounted, but every permission-visible Central
# read model must be warm before the first menu click.
for token in [
    "final Map<int, Widget> _pageCache",
    "_pageCache.putIfAbsent",
    "final targets = <String>{};",
    "targets.add(centralDashboardInitialPath())",
    "targets.add(centralPartnersInitialPath())",
    "targets.add(centralModulesInitialPath())",
    "targets.add(centralModulesCommercialInitialPath())",
    "targets.add(centralPackagesInitialPath())",
    "targets.add(centralPackagesSupplementaryInitialPath())",
    "targets.add(centralFinanceInitialPath())",
    "targets.add(centralImpactInitialPath())",
    "api.prefetch(targets, maxAge: const Duration(seconds: 30))",
]:
    check(token in frontend, f"Central-10.1 Step 5 pre-click warmup contract missing: {token}")

warm_start = frontend.find("void _warmControlPlane()")
warm_end = frontend.find("Future<void> _loadPublishedBrandAssets", warm_start)
warm = frontend[warm_start:warm_end] if warm_start >= 0 and warm_end > warm_start else ""
check("paths.add('/api/v1/billing/plans')" not in warm,
      "Central-10 still contains the old eager raw Billing prefetch storm")
check("paths.add('/api/v1/modules')" not in warm,
      "Central-10 still eagerly prefetches raw module endpoints")
check("String? target;" not in warm,
      "Central-10.1 Step 5 still warms only the current route")

# Browser-side fan-out and obsolete read transforms are forbidden on the core paths.
for token, message in [
    ("Future<void> _loadSupplementary()", "Partner Workspace browser fan-out still exists"),
    ("Future<Map<String, dynamic>?> _safeWorkspaceGet", "Partner Workspace browser service orchestration still exists"),
    ("filteredCommercialRows", "Commercial Matrix still filters/sorts in Flutter"),
    ("commercialSubscription(", "Commercial Matrix still joins subscriptions in Flutter"),
    ("central8LatestWeeklyWindow", "Weekly-window read logic still exists in Flutter"),
    ("central9CanonicalPackage(", "Package pricing authority still exists in Flutter"),
    ("filteredModules", "Partner Workspace still filters modules in Flutter"),
    ("groupedFilteredModules", "Partner Workspace still groups modules in Flutter"),
    ("_partnerModuleSection", "Partner Workspace section classification still lives in Flutter"),
    ("subscriptionFor(", "Partner Workspace still joins subscriptions in Flutter"),
    ("_builtInPartnerCategories", "Partner category fallback still lives in Flutter"),
    ("_mergePartnerCategories", "Partner category merge/order still lives in Flutter"),
    ("workflowCount(", "Finance workflow KPI aggregation still lives in Flutter"),
    ("moneyAcrossCurrencies(", "Finance currency aggregation still lives in Flutter"),
    ("filteredInvoices", "Finance invoice filtering still lives in Flutter"),
    ("revenuePlanKey", "Finance revenue-series business selection still lives in Flutter"),
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
check("api.get('/api/v1/partners/$partnerId'" not in frontend,
      "Partner deep-link loader still performs a legacy pre-read before the Central read model")

# CENTRAL-10.1 Step 1: exact network/cache contract.
for token in [
    "centralDashboardInitialPath()",
    "centralPartnersInitialPath()",
    "centralModulesInitialPath()",
    "centralModulesCommercialInitialPath()",
    "centralPackagesInitialPath()",
    "centralFinanceInitialPath()",
    "centralImpactInitialPath()",
    "void Function(Map<String, dynamic> freshData)? onRefresh",
    "ValueListenable<int> cacheSignal(String path)",
    "_storeCache(path, data, maxAge)",
    "const timeout = Duration(seconds: 4)",
]:
    check(token in frontend, f"Central-10.1 Step 1 network/cache contract missing: {token}")

for obsolete_timeout in [
    "Duration(milliseconds: 950)",
    "Duration(milliseconds: 800)",
]:
    check(obsolete_timeout not in frontend,
          f"Central-10.1 still uses obsolete browser cutoff: {obsolete_timeout}")

for exact_prefetch in [
    "'year': '${DateTime.now().toUtc().year}'",
    "'perspective': 'PARTNER'",
    "'commercial_limit': '120'",
    "'invoice_status': 'ALL'",
    "'revenue_period': 'MONTHLY'",
    "'revenue_plan': 'ALL'",
    "'evidence_limit': '12'",
    "'evidence_offset': '0'",
]:
    check(exact_prefetch in frontend,
          f"Central-10.1 exact warmup/mount cache key missing: {exact_prefetch}")

dashboard_start = gateway_main.find("func (a *app) dashboard(")
dashboard_end = gateway_main.find("\nfunc ", dashboard_start + 1)
dashboard_body = gateway_main[dashboard_start:dashboard_end] if dashboard_start >= 0 and dashboard_end > dashboard_start else ""
check("3*time.Second" not in dashboard_body,
      "Central-10.1 Dashboard still allows a 3-second live read")
check("context.WithTimeout(" not in dashboard_body,
      "Central-10.1 Dashboard request path still waits on a live backend timeout")
check("dashboardSnapshotForRead" in dashboard_body,
      "Central-10.1 Dashboard does not serve the materialized hot snapshot")
check("dashboardRecentActivity" not in dashboard_body,
      "Central-10.1 Dashboard request path still performs live activity I/O")
check('"activity": activityBlock' in dashboard_snapshots,
      "Central-10.1 Dashboard snapshot does not contain precomputed activity")
check("force: loadCategories" not in frontend,
      "Central-10.1 Partners first mount still bypasses warm cache/inflight data")
partners_init = frontend.find("class _PartnersPageState")
partners_load = frontend.find("Future<void> load(", partners_init)
partners_init_block = frontend[partners_init:partners_load] if partners_init >= 0 and partners_load > partners_init else ""
check("load(loadCategories: true);" in partners_init_block and "force: true" not in partners_init_block,
      "Central-10.1 Partners first mount must reuse prefetch/inflight data without a forced duplicate request")
check("central10ReadCache.items = map[string]central10CacheEntry{}" not in gateway,
      "Central-10.1 still globally flushes every Central read cache on mutation")
check("invalidateCentral10Caches(r.URL.Path)" in gateway_main,
      "Central-10.1 mutation invalidation is not route-targeted")

invalidate_start = frontend.find("void _invalidateMutation(String path)")
invalidate_end = frontend.find("Future<Map<String, dynamic>> post(", invalidate_start)
invalidate_body = frontend[invalidate_start:invalidate_end] if invalidate_start >= 0 and invalidate_end > invalidate_start else ""
check("add('/api/v1/central');" not in invalidate_body,
      "Central-10.1 Step 5 still globally clears the browser Central namespace")
for token in [
    "add('/api/v1/central/partners')",
    "add('/api/v1/central/modules')",
    "add('/api/v1/central/modules/commercial')",
    "add('/api/v1/central/packages')",
    "add('/api/v1/central/packages/supplementary')",
    "add('/api/v1/central/finance')",
    "add('/api/v1/central/impact')",
]:
    check(token in invalidate_body, f"Step 5 selective browser invalidation missing: {token}")
check("onRefresh: applyModel" in frontend,
      "Central-10.1 stateful Central pages do not consume SWR refresh callbacks")
check("onRefresh: applyModel" in modules_ui,
      "Central-10.1 Modules page does not consume SWR refresh callbacks")

for token in [
    "Loading the latest partner portfolio snapshot.",
    "Loading the latest finance snapshot.",
    "Loading the latest impact and evidence snapshot.",
]:
    check(token in frontend, f"Central-10.1 Loading != Zero guard missing: {token}")
check("Loading the latest module registry snapshot." in modules_ui,
      "Central-10.1 Modules Loading != Zero guard missing")

# CENTRAL-10.1 Step 3: Modules & Packages must be hot-snapshot/progressive surfaces.
for route in [
    "/api/v1/central/modules/commercial",
    "/api/v1/central/packages/supplementary",
]:
    check(route in gateway, f"Central-10.1 Step 3 route missing in Gateway read model: {route}")
    check(route in gateway_main, f"Central-10.1 Step 3 route missing in top-level API dispatcher: {route}")
    check(f"  {route}:" in openapi, f"Central-10.1 Step 3 OpenAPI path missing: {route}")

modules_start = gateway.find("func (a *app) central10Modules(")
modules_commercial_start = gateway.find("func (a *app) central10ModulesCommercial(", modules_start)
packages_start = gateway.find("func (a *app) central10Packages(", modules_commercial_start)
packages_supp_start = gateway.find("func (a *app) central10PackagesSupplementary(", packages_start)
money_start = gateway.find("func central10MoneyLabel(", packages_supp_start)
modules_primary = gateway[modules_start:modules_commercial_start]
packages_primary = gateway[packages_start:packages_supp_start]

for forbidden in ["internalGET(", "central10AllPartners(", "central10CommercialSources(", "WaitGroup", "wg.Wait()"]:
    check(forbidden not in modules_primary,
          f"Step 3 Modules primary request still blocks on live fan-out: {forbidden}")
for forbidden in ["internalGET(", "WaitGroup", "wg.Wait()"]:
    check(forbidden not in packages_primary,
          f"Step 3 Packages primary request still blocks on live fan-out: {forbidden}")

for token in [
    "central10Step3SnapshotMigration",
    "identity.central_screen_snapshots",
    "refreshCentralStep3Registry",
    "refreshCentralStep3Plans",
    "refreshCentralStep3Analytics",
    "refreshCentralStep3Commercial",
    '"delivery"] = "MATERIALIZED_HOT_SNAPSHOT"',
]:
    check(token in step3_snapshots, f"Step 3 materialized snapshot contract missing: {token}")

for token in [
    "Future<void> loadRegistry()",
    "Future<void> loadCommercial()",
    "/api/v1/central/modules/commercial",
    "commercialLoading",
    "commercialReady",
]:
    check(token in modules_ui, f"Step 3 Modules progressive Flutter contract missing: {token}")

for token in [
    "Future<void> loadSupplementary()",
    "/api/v1/central/packages/supplementary",
    "modulesLoading",
    "Package cards remain usable while analytics loads independently.",
]:
    check(token in frontend, f"Step 3 Packages progressive Flutter contract missing: {token}")

check("String centralModulesInitialPath() => '/api/v1/central/modules';" in frontend,
      "Step 3 Modules prefetch does not target the primary registry snapshot")
check("registry.get(\"modules\")" not in frontend,
      "Step 3 regression guard: unexpected registry transform moved into main Flutter shell")

# CENTRAL-10.1 Step 4: Finance/Impact hot snapshots and dependency-aware readiness.
finance_start = gateway.find("func (a *app) central10Finance(")
impact_start = gateway.find("func (a *app) central10Impact(", finance_start)
partner_module_start = gateway.find("func central10PartnerModuleSection(", impact_start)
finance_handler = gateway[finance_start:impact_start]
impact_handler = gateway[impact_start:partner_module_start]
for forbidden in ["internalGET(", "central10AllPartners(", "sync.WaitGroup", "wg.Wait()", "context.WithTimeout(r.Context()"]:
    check(forbidden not in finance_handler,
          f"Step 4 Finance request path still performs live fan-out: {forbidden}")
    check(forbidden not in impact_handler,
          f"Step 4 Impact request path still performs live fan-out: {forbidden}")

for token in [
    "centralStep4FinanceKey",
    "centralStep4ImpactKey",
    "runCentralStep4Materializer",
    "refreshCentralStep4Finance",
    "refreshCentralStep4Impact",
    "centralStep4AllEvidence",
    '"delivery"] = "MATERIALIZED_HOT_SNAPSHOT"',
]:
    check(token in step4_snapshots, f"Step 4 hot-snapshot contract missing: {token}")

check('r.URL.Path == "/api/v1/central/finance"' in gateway and
      'r.URL.Path == "/api/v1/central/impact"' in gateway,
      "Step 4 Finance/Impact hot routes missing")
check("model['ready'] != true" in frontend,
      "Step 4 Flutter does not preserve Loading != Zero while snapshots warm")
check("healthCheckPath: /api/v1/live" in render,
      "Render deploy gate must use process liveness to avoid downstream-readiness deployment deadlocks")
check("healthCheckPath: /api/v1/health" not in render,
      "Render deploy gate still blocks on full dependency readiness")
check("http.StatusServiceUnavailable" in gateway_main and '"readiness": true' in gateway_main,
      "Step 4 /api/v1/health does not remain fail-closed for dependency diagnostics")

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

# Partner Workspace module read model, Finance read model and partner category
# fallback are authoritative Go responsibilities.
for token in [
    "func central10PartnerModuleView(",
    "func (a *app) central10PartnerModules(",
    '"active_module_keys": activeKeys',
    '"groups": groups',
    "func central10FinanceChart(",
    'row["partner_name"] = name',
    '"outstanding_label"',
    '"paid_ytd_label"',
    "func central10PartnerCategories(",
]:
    check(token in gateway, f"Central-10 backend presentation-model contract missing: {token}")

for token in [
    "List<Map<String, dynamic>> moduleGroups",
    "Map<String, dynamic> moduleKpis",
    "List<String> activeModuleKeys",
    "onChanged: updateModuleQuery",
    "onChanged: (v) => updateModuleState(v ?? 'ALL')",
]:
    check(token in frontend, f"Partner Workspace presentation binding missing: {token}")

check(
    "final path = _financePath();" in frontend and "widget.api.get(\n        path," in frontend,
    "Finance does not load through its parameterized Go read model",
)

for token in [
    "void applyInvoiceFilter(String status)",
    "void applyRevenuePeriod(String period)",
    "void applyRevenuePlan(String planKey)",
    "onSelected: (_) => applyInvoiceFilter(status)",
    "applyRevenuePeriod(value)",
    "applyRevenuePlan(value)",
]:
    check(token in frontend, f"Finance backend filter interaction is not wired end-to-end: {token}")

for marker in [
    "void applyInvoiceFilter(String status)",
    "void applyRevenuePeriod(String period)",
    "void applyRevenuePlan(String planKey)",
]:
    start = frontend.find(marker)
    end = frontend.find("\n  }", start)
    body = frontend[start:end] if start >= 0 and end > start else ""
    check("unawaited(load())" in body,
          f"Finance filter handler changes UI state without reloading the Go read model: {marker}")

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

# Dynamic-N contract: operational page/chunk sizes are allowed, but no total
# partner/module ceiling may truncate the Central dataset.
for token in [
    "func (a *app) central10AllPartners",
    "pageCount := (first.Total + pageSize - 1) / pageSize",
    "central10StringChunks(partnerIDs, 80)",
    "groupLimit := central10PositiveInt",
]:
    check(token in gateway, f"Central-10 dynamic-N contract missing: {token}")
check("central10QueryLimit(r.URL.Query().Get(\"commercial_limit\"), 120, 200)" not in gateway,
      "Commercial Matrix still has a fixed 200-group ceiling")
check("maximum: 200, default: 120" not in openapi,
      "OpenAPI still advertises a fixed 200-group Commercial Matrix ceiling")

# Responsibility score: explicit user-visible read-model capabilities. The
# acceptance floor is 95%; Flutter retains only presentation state and action input.
responsibilities = [
    ("cache/degraded fallback", "central10StaleTTL" in gateway),
    ("bounded backend aggregation", "central10ReadBudget = 650 * time.Millisecond" in gateway),
    ("partner search/filter", 'values.Set("include_stats", "true")' in gateway),
    ("partner KPI aggregation", '"kpis": map[string]any{' in gateway),
    ("partner enrichment join", 'catalogByID :=' in gateway and 'billingByID :=' in gateway),
    ("partner category fallback/merge/order", "central10PartnerCategories" in gateway),
    ("module registry filtering", "registryPreset :=" in gateway),
    ("module KPI aggregation", '"module_registry": len(modules)' in gateway),
    ("commercial search", "commercialQ :=" in gateway),
    ("commercial partner/module/status filter", "statusFilter :=" in gateway),
    ("commercial sorting", "sort.Slice(ids" in gateway and "sort.Slice(keys" in gateway),
    ("commercial grouping", '"modules": rows' in gateway and '"partners": rows' in gateway),
    ("commercial subscription join", 'row["subscription"] = sub' in gateway),
    ("package canonicalization", "central10CanonicalPlan" in gateway),
    ("package analytics materialization", '"/api/v1/billing/packages/analytics"' in step3_snapshots),
    ("finance screen aggregation", "central10Finance" in gateway and '"kpis": kpis' in gateway and "refreshCentralStep4Finance" in step4_snapshots),
    ("finance invoice filter/join", 'row["partner_name"] = name' in gateway and 'invoiceStatus :=' in gateway),
    ("finance revenue chart selection", "central10FinanceChart" in gateway),
    ("finance multi-currency ready labels", "central10MoneyLabel" in gateway),
    ("impact screen aggregation", "central10Impact" in gateway and "refreshCentralStep4Impact" in step4_snapshots),
    ("impact evidence filtering", "central10Step4EvidenceMatches" in gateway),
    ("partner workspace aggregation", "central10PartnerWorkspace" in gateway),
    ("partner workspace module filter/group/KPIs", "central10PartnerModuleView" in gateway),
    ("partner workspace subscription join", 'row["subscription"] = subscription' in gateway),
    ("partner workspace environment selection", "central10ProductionEnvironment" in gateway),
    ("weekly window selection", "central10NormalizeDashboardImpact" in gateway),
    ("truthful data-presence semantics", 'out["has_data"] = false' in gateway and '"has_data":observationCount>0' in impact),
]
backend_units = sum(1 for _, ok in responsibilities if ok)
backend_share = backend_units / len(responsibilities) * 100
check(backend_share >= 95.0,
      f"Central-10 backend read-model responsibility is only {backend_share:.1f}% ({backend_units}/{len(responsibilities)})")
for name, ok in responsibilities:
    check(ok, f"Backend responsibility checkpoint failed: {name}")

# CENTRAL-10.1 Step 5: exact frontend URL runtime proof.
for token in [
    'assert_fast_json "/api/v1/dashboard/summary?year=$YEAR" "Dashboard"',
    'assert_fast_read_model "/api/v1/central/partners?limit=24&offset=0" "Partners"',
    'assert_fast_read_model "/api/v1/central/modules" "Modules Registry"',
    'assert_fast_read_model "/api/v1/central/modules/commercial?perspective=PARTNER&commercial_limit=120" "Modules Commercial / Partner"',
    'assert_fast_read_model "/api/v1/central/packages" "Packages / Plans"',
    'assert_fast_read_model "/api/v1/central/packages/supplementary" "Packages / Supplementary"',
    'assert_fast_read_model "/api/v1/central/finance?invoice_status=ALL&revenue_period=MONTHLY&revenue_plan=ALL" "Finance"',
    'assert_fast_read_model "/api/v1/central/impact?evidence_limit=12&evidence_offset=0" "Impact"',
]:
    check(token in smoke, f"Step 5 exact Flutter runtime URL missing from smoke gate: {token}")
check("invoice_status=PAID&revenue_period=WEEKLY&revenue_plan=ALL" not in smoke,
      "Step 5 smoke still benchmarks a non-initial Finance URL")
check('assert_fast_json "/api/v1/dashboard/summary" "Dashboard"' not in smoke,
      "Step 5 smoke still benchmarks Dashboard without the Flutter year cache key")

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
