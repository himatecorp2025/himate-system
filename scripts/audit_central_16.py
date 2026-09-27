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
round1_ui = read("frontend/lib/central17_round1.dart")
dashboard = read("services/cmd/gateway/dashboard_snapshot.go")
central10 = read("services/cmd/gateway/central10.go")
step4 = read("services/cmd/gateway/central_step4_snapshots.go")
modules_ui = read("frontend/lib/module_control_plane.dart")
website_ui = read("frontend/lib/cms_page.dart")
administration_ui = read("frontend/lib/administration_center.dart")
localization = read("frontend/lib/localization.dart")

# Shared approved visual system.
for token in [
    "const brandNavy = Color(0xFF102642)",
    "const brandGold = Color(0xFFD2A323)",
    "const brandIvory = Color(0xFFF6F8FC)",
    "brightness: Brightness.light",
    "scaffoldBackgroundColor: brandIvory",
    "GoogleFonts.lora",
    "Greater impact.",
    "Stronger communities.",
    "A sustainable future.",
]:
    check(token in frontend, f"CENTRAL-16 shared design contract missing: {token}")
check("'Greater impact.\\nStronger communities.\\nA sustainable future.': 'Nagyobb hatás.\\nErősebb közösségek.\\nFenntartható jövő.'" in localization,
      "CENTRAL-17.1 locale-safe sidebar slogan mapping missing")

# Backend-first architecture remains authoritative.
for token in [
    '"architecture": "GO_BACKEND_READ_MODEL"',
    '"frontend_role": "PRESENTATION_ONLY"',
    "MATERIALIZED_DASHBOARD_SNAPSHOT",
]:
    check(token in central10 or token in dashboard, f"CENTRAL-16 backend-first contract missing: {token}")

# Dashboard visual/functional acceptance.
for token in [
    "class _DashboardUsMapCard",
    "class _DashboardUsMapPainter",
    "class _DashboardPartnerReportPreview",
    "Partners in the United States",
    "PDF export",
]:
    check(token in frontend, f"CENTRAL-16 Dashboard contract missing: {token}")
check("12 month trend" in frontend or "12 month trend" in round1_ui,
      "CENTRAL-16 Dashboard contract missing: 12 month trend")

for token in [
    "func dashboardPartnerGeo(",
    "func dashboardUSStateName(",
    '"active_states"',
    '"active_partners"',
    '"active_modules"',
    '"partner_geo"',
]:
    check(token in dashboard, f"CENTRAL-16 Dashboard backend contract missing: {token}")

# Existing business truth must come from the authoritative Billing plan snapshot,
# not be replaced by mockup/sample pricing in the gateway presentation read model.
for token in [
    "central10PlanDisplayPrice(",
    'central10Float(plan["monthly_price"])',
    'central10Int(plan["module_limit"])',
    'out["entitlement"] = "Unlimited"',
]:
    check(token in central10, f"CENTRAL-16 canonical package contract missing: {token}")
for forbidden in [
    'out["monthly_price"] = 990',
    'out["monthly_price"] = 1490',
    'out["monthly_price"] = 2490',
]:
    check(forbidden not in central10, f"CENTRAL-16 gateway still overrides authoritative package pricing: {forbidden}")

# Partners and overview-first navigation.
for token in [
    "Reference partners",
    "PDF export",
    "New Partner",
]:
    check(token in frontend, f"CENTRAL-16 Partners contract missing: {token}")

# Modules workspace switch and no unbounded warming retry loop.
for token in [
    "class _ModuleWorkspaceTabs",
    "Topics",
    "Partners",
    "Modules",
    "Connections",
    "Module snapshot is warming",
]:
    check(token in modules_ui, f"CENTRAL-16 Modules contract missing: {token}")
check("Future<void>.delayed(const Duration(milliseconds: 350)" not in modules_ui, "CENTRAL-16 Modules still contains automatic 350ms warming polling")
check("Future<void>.delayed(const Duration(milliseconds: 500)" not in modules_ui, "CENTRAL-16 Modules still contains automatic 500ms warming polling")

# Packages, Finance and Impact overview hierarchy.
for token in [
    "All packages",
    "Active subscriptions",
    "Custom packages",
    "Package snapshot is warming",
    "Invoice approval queue",
    "Partner onboarding",
    "New invoice",
    "Active metrics",
    "Pending review",
    "Impact trend",
    "Report creation",
]:
    check(token in frontend, f"CENTRAL-16 overview contract missing: {token}")
check("Future<void>.delayed(const Duration(milliseconds: 350)" not in frontend, "CENTRAL-16 main UI still contains automatic 350ms warming polling")
check("Future<void>.delayed(const Duration(milliseconds: 500)" not in frontend, "CENTRAL-16 main UI still contains automatic 500ms warming polling")

# Impact weekly/monthly analytics must originate in the Go materialized read model.
for token in [
    '"analytics"',
    '"kpis"',
    '"pending_evidence"',
]:
    check(token in central10, f"CENTRAL-16 Impact gateway contract missing: {token}")
for token in [
    "/internal/v1/impact/dashboard?year=",
    '"analytics":',
]:
    check(token in step4, f"CENTRAL-16 Impact materialization contract missing: {token}")

# Website & Marketing control center.
for token in [
    "Design Guide",
    "CMS",
    "SEO",
    "Domain & Deployment",
    "Analytics",
    "Partner Connections",
    "class WebsiteDomainsPanel",
    "class WebsiteAnalyticsPanel",
]:
    check(token in website_ui, f"CENTRAL-16 Website & Marketing contract missing: {token}")

# Administration two-center hierarchy.
for token in [
    "HIMATE Administration Center",
    "Partner Administration Center",
    "System Backup & Recovery",
    "class _AdministrationCenterHeroCard",
    "class _AdministrationQuickCard",
]:
    check(token in administration_ui, f"CENTRAL-16 Administration contract missing: {token}")

# System & Operations executive overview and developer diagnostics.
for token in [
    "System status",
    "Partner systems",
    "Deployments",
    "Issues",
    "class _SystemCurrentHealthCard",
    "class _SystemInfrastructureSummary",
    "Developer diagnostics",
]:
    check(token in frontend, f"CENTRAL-16 System & Operations contract missing: {token}")

# No fixed card-count acceptance. Responsive/dynamic collection rendering stays data driven.
check("for (final plan in canonicalPlans)" in frontend or "for (final plan in plans)" in frontend,
      "CENTRAL-16 package rendering is not dynamic")
check("for (final p in filtered)" in frontend, "CENTRAL-16 partner rendering is not dynamic")

if errors:
    print(f"FAIL: CENTRAL-16 acceptance found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    sys.exit(1)

print("CENTRAL-16 full redesign + backend-first functional acceptance: PASS")
