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
openapi = read("docs/openapi.yaml")

# Approved premium dark / IT-blue visual system.
for token in [
    "const brandNavyDeep = Color(0xFF020914)",
    "const brandSurface = Color(0xFF07182A)",
    "const brandSurfaceRaised = Color(0xFF0B2540)",
    "const brandIonBlue = Color(0xFF19B5FF)",
    "brightness: Brightness.dark",
    "scaffoldBackgroundColor: brandNavyDeep",
    "TweenAnimationBuilder<double>",
]:
    check(token in frontend, f"Central-9 premium visual contract missing: {token}")

# Canonical package display is pinned in the UI and backend migration.
for token in [
    "'STARTER' => <String,dynamic>{'name':'Starter','price':990,'entitlement':'10 modules'}",
    "'BUSINESS' => <String,dynamic>{'name':'Business','price':1490,'entitlement':'20 modules'}",
    "'FLEX' => <String,dynamic>{'name':'Premium','price':2490,'entitlement':'Unlimited'}",
    "central9CanonicalPackagePrice",
]:
    check(token in frontend, f"Central-9 canonical frontend package contract missing: {token}")
for token in [
    "display_name='Starter',monthly_price=990",
    "module_limit=10,selection_mode='FIXED'",
    "display_name='Business',monthly_price=1490",
    "module_limit=20,selection_mode='FIXED'",
    "display_name='Premium',monthly_price=2490",
    "selection_mode='UNLIMITED'",
]:
    check(token in billing, f"Central-9 canonical backend package baseline missing: {token}")

# Weekly Report must show the latest four elapsed weeks, never future generated weeks.
for token in [
    "startOfCurrentWeek",
    "!parsed.isAfter(startOfCurrentWeek)",
    "elapsed.sublist(elapsed.length - 4)",
]:
    check(token in frontend, f"Central-9 four-week elapsed-window contract missing: {token}")
for token in [
    "excludes future weeks and keeps latest four elapsed weeks",
    "currentMonday.subtract(const Duration(days: 21))",
]:
    check(token in frontend_test, f"Central-9 weekly regression test missing: {token}")

# Cache-first / prefetched control-plane loading.
for token in [
    "final Map<String, _ApiCacheEntry> _cache",
    "final Map<String, Future<Map<String, dynamic>>> _inflight",
    "paths.add('/api/v1/billing/plans')",
    "paths.add('/api/v1/billing/packages/analytics')",
    "paths.add('/api/v1/billing/finance/overview')",
    "maxAge: const Duration(seconds: 20)",
    "include_archived=false&include_stats=false",
]:
    check(token in frontend, f"Central-9 performance contract missing: {token}")

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

# Explicit regression tests pin all three prices and entitlements.
for token in [
    "containsPair('price', 990)",
    "containsPair('price', 1490)",
    "containsPair('price', 2490)",
    "containsPair('entitlement', 'Unlimited')",
]:
    check(token in frontend_test, f"Central-9 package regression test missing: {token}")

if errors:
    print(f"FAIL: CENTRAL-9 acceptance found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    sys.exit(1)

print("CENTRAL-9 premium UI, performance, weekly report and PDF-only acceptance: PASS")
