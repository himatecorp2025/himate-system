#!/usr/bin/env python3
from pathlib import Path
import json
import sys

ROOT = Path(__file__).resolve().parents[1]
MATRIX = json.loads((ROOT / "docs/CENTRAL-15_AUDIT_MATRIX.json").read_text())

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit("CENTRAL-15 AUDIT ERROR: " + message)

# Source files used for independent verification.
billing8 = read("services/cmd/billing/central8.go")
frontend = read("frontend/lib/main.dart")
design = read("frontend/lib/design_guide.dart")
backups_main = read("services/cmd/backups/main.go")
backups_worker = read("services/cmd/backups/worker.go")
backups_api = read("services/cmd/backups/api.go")
backups_panel = read("frontend/lib/backups_panel.dart")
restore = read("services/cmd/backups/central14_restore.go")
admin = read("frontend/lib/administration_center.dart")
connections = read("services/cmd/gateway/central13.go")
connections_ui = read("frontend/lib/partner_connections.dart")
localization = read("frontend/lib/localization.dart")

# Positive invariants: these must remain true while we audit gaps.
for token in [
    'centralStep4PartnersKey       = "partners_screen"',
    'centralStep4FinanceKey        = "finance_screen"',
]:
    require(token in read("services/cmd/gateway/central_step4_snapshots.go"),
            f"backend hot snapshot invariant missing: {token}")

for token in [
    "Future<void> openPdfExportIfAvailable(",
    "'No exportable data'",
    "'Nincs exportálható adat'",
]:
    require(token in (frontend + localization), f"PDF empty-state invariant missing: {token}")

for token in [
    "'Primary Color'",
    "'Brand Color'",
    "'Page Background'",
    "'Body Text Color'",
    "Future<void> pickColor(",
    "static const headingFonts",
    "static const bodyFonts",
]:
    require(token in design, f"Design Guide invariant missing: {token}")

for token in [
    "'HIMATE Administration Center'",
    "'Partner Administration Center'",
    "title: 'Financial Administration'",
    "title: 'Corporate Documents'",
    "title: 'System Backup & Recovery'",
    "title: 'Audit & Logs'",
]:
    require(token in admin, f"Administration Center invariant missing: {token}")

for token in [
    '"DELETED"',
    '"SUSPENDED"',
    '"INACTIVE"',
    '"ACTIVE"',
    '"PERSISTED_CONNECTIONS_SCREEN"',
    '"PARTNERS_CONNECTOR_RUNTIME_WEBSITE_ADAPTERS"',
    "centralBrowserMaterializedRead(r)",
    "serveCentral13PersistentConnections",
    "serveLegacyCentral13Connections",
]:
    require(token in connections, f"Partner Connections dual-path invariant missing: {token}")
require("'Partner Data Connections'" in connections_ui,
        "Partner Data Connections UI invariant missing")

# GAP C12-06: industry/category analytics is absent.
package_analytics_start = billing8.find("func (a *app) packageAnalytics(")
package_export_start = billing8.find("func central9PlanPrice", package_analytics_start)
require(package_analytics_start >= 0 and package_export_start > package_analytics_start,
        "Could not isolate packageAnalytics source block")
package_analytics = billing8[package_analytics_start:package_export_start]
industry_present = any(token in package_analytics.lower() for token in [
    '"industry"', '"industry_count"', '"industries"', '"category_id"', '"category_name"',
])
gap_c12_06 = not industry_present

# GAP C13-07: real preview is opened externally, while inline preview remains schematic.
gap_c13_07 = (
    "openBrowserDownload(path);" in design
    and "Click the preview to open the real draft website in a new tab." in design
    and "HtmlElementView" not in design
    and "IFrameElement" not in design
)

# GAP C14-06: scheduler only considers existing policies, no partner-registry reconciliation.
scheduler_start = backups_worker.find("func (a *app) runSchedulerOnce()")
scheduler_end = backups_worker.find("func (a *app) scheduler()", scheduler_start)
require(scheduler_start >= 0 and scheduler_end > scheduler_start,
        "Could not isolate backup scheduler")
scheduler = backups_worker[scheduler_start:scheduler_end]
gap_c14_06 = (
    "FROM backups.policies WHERE enabled=TRUE" in scheduler
    and "partners" not in scheduler.lower()
)
# Lazy policy creation alone is not automatic enrollment.
require("func (a *app) ensurePolicy(" in backups_main,
        "ensurePolicy implementation unexpectedly missing")

# GAP C14-08: backend can list history but production restore UI selects only latest summary point.
gap_c14_08 = (
    "case http.MethodGet:" in backups_api
    and "ORDER BY created_at DESC" in backups_api
    and "latest_restore_point_id" in backups_panel
    and "Future<void> _restoreProduction(" in backups_panel
    and "restorePointHistory" not in backups_panel
    and "selectedRestorePoint" not in backups_panel
)

# GAP C14-09: platform backup exists but executable production platform restore does not.
gap_c14_09 = (
    'VALUES(\'_platform\',30,30,24,TRUE)' in backups_main
    and "platform production restore is maintenance-only" in restore
)
maintenance_artifacts = [
    ROOT / "scripts" / "restore_platform.sh",
    ROOT / "scripts" / "platform_restore.sh",
    ROOT / "scripts" / "recover_platform.sh",
    ROOT / "docs" / "PLATFORM_RECOVERY.md",
    ROOT / "docs" / "PLATFORM_RESTORE.md",
]
if any(p.exists() for p in maintenance_artifacts):
    gap_c14_09 = False

computed = {
    "C12-06": gap_c12_06,
    "C13-07": gap_c13_07,
    "C14-06": gap_c14_06,
    "C14-08": gap_c14_08,
    "C14-09": gap_c14_09,
}

matrix_status = {item["id"]: item["status"] for item in MATRIX["requirements"]}
for gap_id, present in computed.items():
    require(matrix_status.get(gap_id) == "GAP",
            f"{gap_id} is not recorded as GAP in the audit matrix")
    require(present, f"{gap_id} no longer reproduces; update CENTRAL-15 matrix and closure audit")

total = len(MATRIX["requirements"])
gaps = [item for item in MATRIX["requirements"] if item["status"] == "GAP"]
passes = [item for item in MATRIX["requirements"] if item["status"] == "PASS"]

print(f"CENTRAL-15 audit matrix verified: {total} requirements")
print(f"PASS={len(passes)} GAP={len(gaps)}")
for item in gaps:
    print(f"GAP {item['id']} [{item['round']}]: {item['requirement']}")
    print(f"  {item.get('finding','')}")
print("CENTRAL-15 is NOT CLOSED while GAP > 0.")

# Intentional non-zero exit: this audit is a closure gate, not a cosmetic report.
sys.exit(2 if gaps else 0)
