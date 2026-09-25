#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

frontend = read("frontend/lib/main.dart")
impact = read("services/cmd/impact/main.go")
gateway = read("services/cmd/gateway/main.go")
localization = read("frontend/lib/localization.dart")

for target in [
    "onTap:canNavigate(1)?()=>onNavigate(1):null",
    "onTap:canNavigate(2)?()=>onNavigate(2):null",
    "onTap:canNavigate(4)?()=>onNavigate(4):null",
    "onTap:canNavigate(5)?()=>onNavigate(5):null",
]:
    require(target in frontend, f"Dashboard KPI navigation missing: {target}")

require("final VoidCallback? onTap;" in frontend, "KPI cards do not expose an interactive callback")
require("weeklyTrend=items(<String,dynamic>{'items':impact['weekly_trend']})" in frontend,
        "Dashboard does not consume weekly Impact data")
require("DropdownMenuItem(value:true,child:LText(uiLiteral('Weekly')))" in frontend,
        "Program Impact Weekly selector is missing")
require("oldDelegate.labels.toString()!=labels.toString()" in frontend,
        "Impact chart does not repaint when period labels change")
require("'Weekly': 'Heti'" in localization, "Weekly dashboard view is not bilingual")

require("date_trunc('week',v.period_end)" in impact, "Impact service does not aggregate by week")
require('"weekly_trend":weeklyTrend' in impact, "Impact service does not expose weekly_trend")
require(impact.count("COALESCE(p.test_partner,FALSE)=FALSE") >= 3,
        "Dashboard Impact aggregation must continue excluding test partners")

require("FROM identity.audit_events" in gateway, "Recent Activity is not backed by the append-only audit store")
require("WHERE outcome='SUCCESS'" in gateway, "Recent Activity is not restricted to successful audit events")
require('"source":"IDENTITY_APPEND_ONLY_AUDIT"' in gateway,
        "Dashboard does not identify the Recent Activity authoritative source")
require('"weekly_trend":weeklyTrend' in gateway,
        "Gateway degraded Dashboard contract does not preserve weekly_trend")

print("Central-2 Dashboard acceptance: PASS")
