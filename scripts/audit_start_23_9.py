#!/usr/bin/env python3
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    p = ROOT / path
    if not p.exists():
        raise SystemExit(f"START-23.9 audit failed: missing {path}")
    return p.read_text(encoding="utf-8")

def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit("START-23.9 audit failed: " + message)

billing = read("services/cmd/billing/main.go")
impact = read("services/cmd/impact/main.go")
gateway = read("services/cmd/gateway/main.go")
frontend = read("frontend/lib/main.dart")
matrix = json.loads(read("docs/START-23.1_FUNCTIONAL_MATRIX.json"))
ci = read(".github/workflows/ci.yml")
compose = read("docker-compose.yml")
render = read("render.yaml")
openapi = read("docs/openapi.yaml")
acceptance = read("docs/START-23.9_ACCEPTANCE.md")
smoke = read("scripts/smoke_start_23_9.sh")

require("0.8.13-start-23.9" in compose, "Compose release version is not START-23.9")
require("0.8.13-start-23.9" in render, "Render release version is not START-23.9")
require("version: 0.8.13-start-23.9" in openapi, "OpenAPI release version is not START-23.9")

for token in (
    '"/internal/v1/analytics/dashboard"',
    "dashboardAnalytics",
    "billing.initial_licenses",
    "billing.invoices",
    "status='PAID'",
    "paid_at",
    '"currency_policy": "NO_FX_CONVERSION"',
):
    require(token in billing, f"Billing Dashboard analytics missing {token!r}")

for token in (
    'dashboardPeopleMetricKey = "klavierhaus.events.attendance.attendee_count"',
    '"/internal/v1/impact/dashboard"',
    "dashboardImpact",
    "people_reached_ytd",
    "impact.metric_values",
):
    require(token in impact, f"Impact Dashboard analytics missing {token!r}")

for token in (
    'path == "/api/v1/dashboard/summary", path == "/api/v1/search"',
    "dashboardRecentActivity",
    "IDENTITY_APPEND_ONLY_AUDIT",
    "permissionResource+\".read\"",
    "func (a *app) globalSearch",
    '"permission_scoped":true',
    'a.hasPermission(actor,"partners.read")',
    'a.hasPermission(actor,"catalog.read")',
    'a.hasPermission(actor,"contact.read")',
    'a.hasPermission(actor,"cms.read")',
    'a.hasPermission(actor,"administration.read")',
    'a.hasPermission(actor,"audit.read")',
    "cacheable:=year==time.Now().UTC().Year()",
):
    require(token in gateway, f"Gateway START-23.9 contract missing {token!r}")

for stale in (
    "Billing analytics upcoming",
    "Impact data in START-13",
    "Global search will be activated in a later functional cycle.",
    "final vals=<double>[.12,.26,.20,.37,.49,.39,.53,.48,.61,.70,.68,.84]",
    "New partner registered",
):
    require(stale not in frontend, f"stale Dashboard/search placeholder remains: {stale!r}")

for token in (
    "_GlobalSearchDialog",
    "'/api/v1/search'",
    "people_reached_ytd",
    "_dashboardMoney",
    "_ImpactChartPainter",
    "Live audit feed",
):
    require(token in frontend, f"Flutter START-23.9 wiring missing {token!r}")

completed = tuple(int(x) for x in str(matrix.get("completed_through", "0")).split("."))
require(completed >= (23, 9), "functional matrix is not completed through START-23.9")
rows = [x for x in matrix.get("contracts", []) if x.get("target_phase") == "23.9"]
require(len(rows) == 5, f"expected 5 START-23.9 contracts, found {len(rows)}")
for row in rows:
    require(row.get("current_state") == "MUTATION_PROVEN_PROD_UNVERIFIED",
            f"{row.get('id')} is not proven")
    require("scripts/smoke_start_23_9.sh" in str(row.get("e2e_proof", "")),
            f"{row.get('id')} lacks START-23.9 E2E proof")

for path in ("/api/v1/dashboard/summary:", "/api/v1/search:"):
    require(path in openapi, f"OpenAPI missing {path}")

for token in (
    "audit_start_23_8.py",
    "audit_start_23_9.py",
    "smoke_start_23_8.sh",
    "smoke_start_23_9.sh",
    "docs/START-23.9_ACCEPTANCE.md",
):
    require(token in ci, f"CI does not retain required gate {token}")

require("HIMATE START-23.9 Dashboard, Analytics & Global Search smoke passed" in smoke,
        "START-23.9 Compose smoke completion marker is missing")
require("No START-23.10+ product scope is included." in acceptance,
        "START-23.9 acceptance does not preserve the phase boundary")

print("START-23.9 static audit passed: 5/5 Dashboard, Analytics & Global Search contracts closed")
