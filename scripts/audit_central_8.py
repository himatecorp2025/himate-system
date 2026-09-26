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
frontend_test = read("frontend/test/central8_manual_qa_test.dart")
localization = read("frontend/lib/localization.dart")
billing = read("services/cmd/billing/central8.go")
billing_main = read("services/cmd/billing/main.go")
billing_c6 = read("services/cmd/billing/central6.go")
gateway = read("services/cmd/gateway/central8.go")
gateway_c10 = read("services/cmd/gateway/central10.go")
gateway_main = read("services/cmd/gateway/main.go")
partner_portal = read("services/cmd/gateway/partner_portal.go")
partners = read("services/cmd/partners/central8.go")
partners_main = read("services/cmd/partners/main.go")
impact = read("services/cmd/impact/central8.go")
impact_main = read("services/cmd/impact/main.go")
ci = read(".github/workflows/ci.yml")
acceptance = read("docs/CENTRAL-8_ACCEPTANCE.md")

check(bool(acceptance.strip()), "CENTRAL-8 acceptance document is empty")

# Canonical package definition must be reapplied in a new migration so deployed DBs are corrected.
for token in [
    "Version: 19",
    "central-8-package-canonical-baseline-and-analytics",
    "display_name='Starter',monthly_price=990",
    "module_limit=10,selection_mode='FIXED'",
    "display_name='Business',monthly_price=1490",
    "module_limit=20,selection_mode='FIXED'",
    "display_name='Premium',monthly_price=2490",
    "selection_mode='UNLIMITED'",
]:
    check(token in billing, f"canonical package migration contract missing: {token}")
check("central8BillingMigration()," in billing_main, "CENTRAL-8 Billing migration is not registered")

# Package Analytics must use authoritative persisted sources, never duplicated frontend blobs.
for token in [
    "billing.partner_plan_subscriptions",
    "catalog.module_usage_events",
    "identity.partner_portal_activity_buckets",
    "billing.partner_onboarding",
    "billing.partner_terms",
]:
    check(token in billing, f"Package Analytics authoritative source missing: {token}")
for token in [
    '"/api/v1/billing/packages/analytics"',
    '"/api/v1/billing/packages/export.pdf"',
    '"/api/v1/billing/finance/export.pdf"',
]:
    check(token in billing_main, f"Billing CENTRAL-8 endpoint is not registered: {token}")

# Partner Portal active-time telemetry is real data, not a fabricated duration.
for token in [
    "Version: 16",
    "partner_portal_activity_buckets",
    "bucket_start",
    "request_count",
]:
    check(token in gateway, f"Partner Portal activity telemetry missing: {token}")
check("central8GatewayMigration()," in gateway_main, "CENTRAL-8 Gateway migration is not registered")
check(partner_portal.count("recordPartnerPortalActivity(u)") >= 2,
      "Partner Portal activity is not recorded at login and authenticated API use")
check("5-minute buckets" in billing, "Package Analytics does not disclose active-time bucket semantics")

# Bulk export is backend-generated, branded PDF and route-backed (Central-9 supersedes Central-8 CSV transport).
check('"/api/v1/partners/export.pdf"' in partners_main, "Partners CSV route is not registered")
check("exportPartnersPDF" in partners and "WriteBrandedTablePDF" in partners, "Partners branded PDF backend is missing")
check('"/api/v1/impact/export.pdf"' in impact_main, "Impact CSV route is not registered")
check("exportImpactPDF" in impact and "WriteBrandedTablePDF" in impact, "Impact branded PDF backend is missing")
for token in [
    "/api/v1/partners/export.pdf",
    "/api/v1/billing/packages/export.pdf",
    "/api/v1/billing/finance/export.pdf",
    "/api/v1/impact/export.pdf",
]:
    check(token in frontend, f"Central UI does not expose functional export path: {token}")

# Loading behavior: Central-8 progressive loading remains valid, but Central-10
# may supersede browser fan-out with one bounded Go read model.
legacy_progressive = all(token in frontend for token in [
    "Future<Map<String, dynamic>?> _safeWorkspaceGet",
    ".timeout(const Duration(seconds: 8))",
    "Future<void> _loadSupplementary()",
    "unawaited(_loadSupplementary())",
])
backend_first = all(token in gateway_c10 for token in [
    "central10ReadBudget = 650 * time.Millisecond",
    'case strings.HasPrefix(r.URL.Path, "/api/v1/central/partners/")',
    '"frontend_role": "PRESENTATION_ONLY"',
])
check(legacy_progressive or backend_first,
      "bounded Partner Detail loading contract is missing")
check("No retry loop is started for an empty dataset." in frontend,
      "Package Analytics empty-state contract is missing")
check("No impact data" in frontend or "No monthly impact data recorded yet." in frontend,
      "Impact empty-state contract is missing")

# Functional acceptance: Central-10 moves weekly-window and partner filtering
# to Go. Do not require obsolete Flutter data transforms to remain.
legacy_weekly = (
    "central8LatestWeeklyWindow" in frontend
    and "elapsed.sublist(elapsed.length - 4)" in frontend
    and "!parsed.isAfter(startOfCurrentWeek)" in frontend
)
backend_weekly = all(token in gateway_c10 for token in [
    "central10NormalizeDashboardImpact",
    "if len(elapsed) > 4 { elapsed = elapsed[len(elapsed)-4:] }",
    'out["weekly_trend"] = elapsed',
])
check(legacy_weekly or backend_weekly,
      "latest-four elapsed-week contract is missing")
check(
    "Central-8 KPI action is applied on the first click" in frontend_test,
    "CENTRAL-8 first-click KPI presentation regression test missing",
)
check(
    "Central-8 responsive presentation grid remains deterministic" in frontend_test,
    "CENTRAL-8 responsive presentation regression test missing",
)

# Route-state persistence must update the browser URL when Central navigation changes.
for token in [
    "String _routeForIndex(int index)",
    "html.window.history.replaceState(null, '', route)",
]:
    check(token in frontend, f"Central route-state persistence missing: {token}")

# Finance analytics must expose weekly/monthly and plan-specific paid revenue.
for token in [
    '"weekly_paid":weekly',
    '"monthly_paid_by_plan":monthlyByPlan',
    '"weekly_paid_by_plan":weeklyByPlan',
]:
    check(token in billing_c6, f"Finance trend response missing: {token}")
for token in [
    "revenuePeriod",
    "revenuePlanKey",
    "weekly_paid_by_plan",
    "monthly_paid_by_plan",
]:
    check(token in frontend, f"Finance analytics UI contract missing: {token}")

# Package Definition and Package Analytics are separate surfaces.
check("title: 'Package Analytics'" in frontend, "Package Analytics surface is missing")
check("_PackageOverviewCard" in frontend, "Canonical Package Definition cards are missing")
check("duplicate package" not in frontend.lower(), "Unexpected duplicate-package placeholder remains")

# New manual-QA strings must participate in the bilingual literal system.
for token in [
    "Package Analytics",
    "Export PDF",
    "No package analytics yet",
]:
    check(token in localization or token in frontend, f"CENTRAL-8 UI label disappeared: {token}")

# CI order must be deterministic and must not skip CENTRAL-8.
static7 = ci.find("python3 scripts/audit_central_7.py")
static8 = ci.find("python3 scripts/audit_central_8.py")
build = ci.find("Build and start containerized microservices")
runtime7 = ci.find("sh scripts/smoke_central_7.sh http://127.0.0.1:8080")
runtime8 = ci.find("sh scripts/smoke_central_8.sh http://127.0.0.1:8080")
check(static7 >= 0, "CENTRAL-7 static gate disappeared from CI")
check(static8 >= 0, "CENTRAL-8 static gate is missing from CI")
check(build >= 0, "Compose build gate is missing from CI")
if static7 >= 0 and static8 >= 0:
    check(static7 < static8, "CENTRAL-8 static gate must run after CENTRAL-7")
if static8 >= 0 and build >= 0:
    check(static8 < build, "CENTRAL-8 static gate must run before Compose build")
check(runtime7 >= 0, "CENTRAL-7 runtime gate disappeared from CI")
check(runtime8 >= 0, "CENTRAL-8 runtime gate is missing from CI")
if runtime7 >= 0 and runtime8 >= 0:
    check(runtime7 < runtime8, "CENTRAL-8 runtime gate must run after CENTRAL-7")

if errors:
    print(f"FAIL: CENTRAL-8 static acceptance found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    sys.exit(1)

print("CENTRAL-8 manual QA, analytics, export and loading static acceptance: PASS")
