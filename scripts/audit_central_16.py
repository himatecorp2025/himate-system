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
dashboard = read("services/cmd/gateway/dashboard_snapshot.go")
central10 = read("services/cmd/gateway/central10.go")

# Shared approved visual system.
for token in [
    "const brandNavy = Color(0xFF102642)",
    "const brandGold = Color(0xFFD2A323)",
    "const brandIvory = Color(0xFFF6F8FC)",
    "brightness: Brightness.light",
    "scaffoldBackgroundColor: brandIvory",
    "GoogleFonts.cormorantGaramond",
    "Nagyobb hatás.",
    "Erősebb közösségek.",
    "Fenntartható jövő.",
]:
    check(token in frontend, f"CENTRAL-16 shared design contract missing: {token}")

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
    "12 month trend",
    "PDF export",
]:
    check(token in frontend, f"CENTRAL-16 Dashboard contract missing: {token}")

for token in [
    "func dashboardPartnerGeo(",
    "func dashboardUSStateName(",
    '"active_states"',
    '"active_partners"',
    '"active_modules"',
    '"partner_geo"',
]:
    check(token in dashboard, f"CENTRAL-16 Dashboard backend contract missing: {token}")

# Existing business truth must not be replaced by mockup sample pricing.
for token in [
    'out["display_price"] = "$990 + VAT"',
    'out["display_price"] = "$1,490 + VAT"',
    'out["display_price"] = "$2,490 + VAT"',
    'out["entitlement"] = "Unlimited"',
]:
    check(token in central10, f"CENTRAL-16 canonical package contract missing: {token}")

# No fixed card-count acceptance. Responsive/dynamic collection rendering stays data driven.
check("for (final plan in plans)" in frontend, "CENTRAL-16 package rendering is not dynamic")
check("for (final p in filtered)" in frontend, "CENTRAL-16 partner rendering is not dynamic")

if errors:
    print(f"FAIL: CENTRAL-16 acceptance found {len(errors)} issue(s)")
    for error in errors:
        print(" -", error)
    sys.exit(1)

print("CENTRAL-16 design foundation + Dashboard/backend-first acceptance: PASS")
