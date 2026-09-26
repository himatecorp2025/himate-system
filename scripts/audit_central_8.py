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
    '"/api/v1/billing/packages/export.csv"',
    '"/api/v1/billing/finance/export.csv"',
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

# Bulk export is backend-generated and route-backed.
check('"/api/v1/partners/export.csv"' in partners_main, "Partners CSV route is not registered")
check("encoding/csv" in partners and "exportPartnersCSV" in partners, "Partners CSV backend is missing")
check('"/api/v1/impact/export.csv"' in impact_main, "Impact CSV route is not registered")
check("encoding/csv" in impact and "exportImpactCSV" in impact, "Impact CSV backend is missing")
for token in [
    "/api/v1/partners/export.csv",
    "/api/v1/billing/packages/export.csv",
    "/api/v1/billing/finance/export.csv",
    "/api/v1/impact/export.csv",
]:
    check(token in frontend, f"Central UI does not expose functional export path: {token}")

# Loading behavior: primary Partner Detail paint must not wait for all secondary services.
for token in [
    "Future<Map<String, dynamic>?> _safeWorkspaceGet",
    ".timeout(const Duration(seconds: 8))",
    "Future<void> _loadSupplementary()",
    "unawaited(_loadSupplementary())",
    "Available sections were loaded independently",
]:
    check(token in frontend, f"bounded/progressive Partner Detail loading contract missing: {token}")
check("No retry loop is started for an empty dataset." in frontend,
      "Package Analytics empty-state contract is missing")
check("No impact data" in frontend, "Impact empty-state contract is missing")

# Functional acceptance: first-click filters and latest four weekly observations are executable tests.
for token in [
    "central8LatestWeeklyWindow",
    "central8PartnerPresetRows",
]:
    check(token in frontend, f"CENTRAL-8 functional helper missing: {token}")
for token in [
    "weekly Dashboard window shows the latest four real weeks",
    "KPI applies partner filtering on the first click",
    "central8PartnerPresetRows",
]:
    check(token in frontend_test, f"CENTRAL-8 Flutter acceptance test missing: {token}")
check("rows.sublist(rows.length - 4)" in frontend,
      "Dashboard weekly window is no longer constrained to the latest four observations")

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
    "Export CSV",
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
