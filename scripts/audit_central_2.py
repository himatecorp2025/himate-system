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
dashboard_snapshot = read("services/cmd/gateway/dashboard_snapshot.go")

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
require(
    '"weekly_trend":weeklyTrend' in gateway or '"weekly_trend":[]any{}' in gateway or
    "central10NormalizeDashboardImpact" in dashboard_snapshot,
    "Gateway degraded Dashboard contract does not preserve weekly_trend",
)

# CENTRAL-10.1 Step 2: Dashboard GET must be a hot snapshot read, never live fan-out.
dashboard_start = gateway.index("func (a *app) dashboard(w http.ResponseWriter")
dashboard_end = gateway.index("func searchText", dashboard_start)
dashboard_handler = gateway[dashboard_start:dashboard_end]
require("internalGET(" not in dashboard_handler, "Dashboard request path still performs live service fan-out")
require("sync.WaitGroup" not in dashboard_handler, "Dashboard request path still blocks on live aggregation")
require("dashboardSnapshotForRead" in dashboard_handler, "Dashboard does not read the materialized hot snapshot")
require("requestDashboardRefresh" in dashboard_handler, "Dashboard stale/refresh state does not queue background refresh")

for token in [
    "identity.dashboard_snapshots",
    "runDashboardMaterializer",
    "materializeDashboardSnapshot",
    "dashboardStaleBlock",
    '"architecture": "MATERIALIZED_DASHBOARD_SNAPSHOT"',
]:
    require(token in dashboard_snapshot, f"Materialized Dashboard contract missing: {token}")

require('partnerBlock = dashboardStaleBlock(previous["partners"])' in dashboard_snapshot,
        "Partner partial failure does not preserve last-known-good state")
require('moduleBlock = dashboardStaleBlock(previous["modules"])' in dashboard_snapshot,
        "Module partial failure does not preserve last-known-good state")
require('billingBlock := dashboardStaleBlock(previous["billing"])' in dashboard_snapshot,
        "Billing partial failure does not preserve last-known-good state")
require('impactBlock := dashboardStaleBlock(previous["impact"])' in dashboard_snapshot,
        "Impact partial failure does not preserve last-known-good state")

for target in [
    "onTap: canNavigate(1) ? () => onNavigate(1) : null",
    "onTap: canNavigate(2) ? () => onNavigate(2) : null",
    "onTap: canNavigate(4) ? () => onNavigate(4) : null",
    "onTap: canNavigate(5) ? () => onNavigate(5) : null",
]:
    require(target in frontend, f"Loading-state Dashboard KPI is not clickable: {target}")

require("partnersAvailable?'${p['live']??0}':'—'" in frontend,
        "Dashboard partner outage can still render a false business zero")
require("modulesAvailable?'${m['catalog_total']??0}':'—'" in frontend,
        "Dashboard module outage can still render a false business zero")
require("!billingAvailable?'—'" in frontend,
        "Dashboard billing outage can still render a false revenue zero")
require("!impactAvailable" in frontend and "?'—'" in frontend,
        "Dashboard impact outage can still render a false people-reached zero")

print("Central-2 Dashboard acceptance: PASS")
