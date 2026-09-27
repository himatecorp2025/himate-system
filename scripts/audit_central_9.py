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
localization = read("frontend/lib/localization.dart")
frontend_test = read("frontend/test/central9_premium_test.dart")
pdf = read("services/internal/common/branded_pdf.go")
partners = read("services/cmd/partners/central8.go")
partners_main = read("services/cmd/partners/main.go")
billing = read("services/cmd/billing/central8.go")
billing_main = read("services/cmd/billing/main.go")
impact = read("services/cmd/impact/central8.go")
impact_main = read("services/cmd/impact/main.go")
gateway_c10 = read("services/cmd/gateway/central10.go")
openapi = read("docs/openapi.yaml")

# CENTRAL-16 intentionally supersedes the Central-9 dark presentation layer
# while preserving the architecture, performance, export and business contracts.
# Accept the approved light canvas + dark navy navigation system.
for token in [
    "const brandNavyDeep = Color(0xFF071A2E)",
    "const brandSurface = Color(0xFFFFFFFF)",
    "const brandSurfaceRaised = Color(0xFFFFFFFF)",
    "const brandIonBlue = Color(0xFF1769E0)",
    "brightness: Brightness.light",
    "scaffoldBackgroundColor: brandIvory",
    "TweenAnimationBuilder<double>",
]:
    check(token in frontend, f"Central-16 visual contract missing while running Central-9 regression: {token}")

# Canonical package identity remains stable, while current commercial values
# come from the authoritative Billing snapshot instead of gateway literals.
backend_readmodel_packages = all(token in gateway_c10 for token in [
    "central10PlanDisplayPrice(",
    'central10Float(plan["monthly_price"])',
    'central10Int(plan["module_limit"])',
    'out["display_name"] = "Starter"',
    'out["display_name"] = "Business"',
    'out["display_name"] = "Premium"',
    'out["entitlement"] = "Unlimited"',
])
check(backend_readmodel_packages,
      "Central-9 canonical package display/read-model contract missing")
for forbidden in [
    'out["monthly_price"] = 990',
    'out["monthly_price"] = 1490',
    'out["monthly_price"] = 2490',
]:
    check(forbidden not in gateway_c10,
          f"Central-9 gateway overrides authoritative Billing price: {forbidden}")
for token in [
    "display_name='Starter',monthly_price=990",
    "module_limit=10,selection_mode='FIXED'",
    "display_name='Business',monthly_price=1490",
    "module_limit=20,selection_mode='FIXED'",
    "display_name='Premium',monthly_price=2490",
    "selection_mode='UNLIMITED'",
]:
    check(token in billing, f"Central-9 canonical backend package baseline missing: {token}")

# Weekly report/window authority may live in Flutter (Central-9) or Go
# (Central-10), but it must always exclude future weeks and retain four elapsed.
legacy_weekly = all(token in frontend for token in [
    "startOfCurrentWeek",
    "!parsed.isAfter(startOfCurrentWeek)",
    "elapsed.sublist(elapsed.length - 4)",
])
backend_weekly = all(token in gateway_c10 for token in [
    "central10NormalizeDashboardImpact",
    "parsed.After(startOfCurrentWeek)",
    "elapsed[len(elapsed)-4:]",
])
check(legacy_weekly or backend_weekly,
      "Central-9 four-week elapsed-window contract missing")

# Central-10.1 keeps widgets lazily mounted while prewarming every permission-
# visible materialized screen read model before the first menu click.
legacy_prefetch = all(token in frontend for token in [
    "final Map<String, _ApiCacheEntry> _cache",
    "final Map<String, Future<Map<String, dynamic>>> _inflight",
    "paths.add('/api/v1/billing/plans')",
    "paths.add('/api/v1/billing/packages/analytics')",
])
backend_first_prefetch = all(token in frontend for token in [
    "final Map<int, Widget> _pageCache",
    "final primaryTargets = <String>{};",
    "final deferredTargets = <String>{};",
    "primaryTargets.add(centralModulesInitialPath())",
    "primaryTargets.add(centralPackagesInitialPath())",
    "primaryTargets.add(centralFinanceInitialPath())",
    "deferredTargets.add(centralImpactInitialPath())",
    "api.prefetch(primaryTargets, maxAge: const Duration(seconds: 30))",
    "api.prefetch(deferredTargets, maxAge: const Duration(seconds: 30))",
])
check(legacy_prefetch or backend_first_prefetch,
      "Central-9/10 performance contract missing")

# PDF only: no CSV export route remains on Central surfaces.
routes = {
    "partners": (partners_main, "/api/v1/partners/export.pdf", partners, "exportPartnersPDF"),
    "packages": (billing_main, "/api/v1/billing/packages/export.pdf", billing, "packageExportPDF"),
    "finance": (billing_main, "/api/v1/billing/finance/export.pdf", billing, "financeExportPDF"),
    "impact": (impact_main, "/api/v1/impact/export.pdf", impact, "exportImpactPDF"),
}
for name, (main, route, implementation, handler) in routes.items():
    check(route in main, f"Central-9 {name} PDF route missing")
    check(handler in implementation, f"Central-9 {name} PDF handler missing")
    check(route in frontend or name == "finance", f"Central-9 {name} frontend PDF path missing")

for path, content in [
    ("frontend/lib/main.dart", frontend),
    ("services/cmd/partners/main.go", partners_main),
    ("services/cmd/billing/main.go", billing_main),
    ("services/cmd/impact/main.go", impact_main),
    ("docs/openapi.yaml", openapi),
]:
    check("export.csv" not in content, f"Legacy CSV export route remains in {path}")

for token in [
    "WriteBrandedTablePDF",
    "%PDF-1.4",
    "HiMate Central - confidential operational export",
]:
    check(token in pdf, f"Branded PDF renderer contract missing: {token}")
check("'Export PDF': 'PDF exportálása'" in localization, "Export PDF is missing Hungarian localization")
check("application/pdf:" in openapi, "OpenAPI does not advertise application/pdf")
check("text/csv:" not in openapi, "OpenAPI still advertises text/csv for Central exports")

# Regression tests remain presentation-focused after Central-10 moved canonical
# prices and weekly selection to Go.
for token in [
    "Central-16 approved light navy-gold visual contract supersedes Central-9 dark presentation",
    "Central responsive presentation helpers remain deterministic",
]:
    check(token in frontend_test, f"Central-9 presentation regression test missing: {token}")


if errors:
    print(f"FAIL: CENTRAL-9 acceptance found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    sys.exit(1)

print("CENTRAL-9 premium UI, performance, weekly report and PDF-only acceptance: PASS")
