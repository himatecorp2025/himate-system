#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LIB = ROOT / "frontend" / "lib"
TEST = ROOT / "frontend" / "test"

errors = []

main = (LIB / "main.dart").read_text()
admin = (LIB / "administration_rbac.dart").read_text()
design = (LIB / "design_guide.dart").read_text()
cms = (LIB / "cms_page.dart").read_text()
notifications = (LIB / "notifications_panel.dart").read_text()
portal = (LIB / "partner_portal.dart").read_text()
localization = (LIB / "localization.dart").read_text()
responsive_test = (TEST / "start23_responsive_qa_test.dart").read_text()

required_main = [
    "bool useCompactLoginForSize(Size size)",
    "int responsiveGridColumnsForWidth(double width)",
    "bool shouldStackContentActions(double width, int actionCount)",
    "class ResponsiveActionBar extends StatelessWidget",
    "useCompactLoginForSize(Size(constraints.maxWidth, constraints.maxHeight))",
    "responsiveGridColumnsForWidth(constraints.maxWidth)",
    "shouldStackContentActions(constraints.maxWidth, actions.length)",
    "maxHeight: viewport.height * (phone ? .94 : .88)",
]
for token in required_main:
    if token not in main:
        errors.append(f"main.dart missing START-23 responsive primitive: {token}")

if "trailing: Row(" in admin:
    errors.append("Administration section actions still use a non-wrapping trailing Row")
if "ResponsiveActionBar(" not in design:
    errors.append("Design Guide save/publish actions are not using ResponsiveActionBar")
if cms.count("ResponsiveActionBar(") < 2:
    errors.append("CMS editor header/footer controls are not fully responsive")
if "Mark all read" in notifications and "child: Row(children: [" in notifications[
    notifications.find("Mark all read") - 700 : notifications.find("Mark all read") + 100
]:
    errors.append("Notification filter controls still use a fixed Row")
if "overflow: TextOverflow.ellipsis" not in portal or "company['display_name']" not in portal:
    errors.append("Partner Portal long organization name protection is missing")

for path in LIB.glob("*.dart"):
    if path.name == "main.dart":
        content = main
    else:
        content = path.read_text()
    if "DataTable(" in content:
        errors.append(
            f"{path.name}: raw DataTable is prohibited by START-23; use responsive record cards or an explicitly adaptive surface"
        )

viewport_tokens = [
    "Size(320, 568)",
    "Size(390, 844)",
    "Size(768, 1024)",
    "Size(1024, 768)",
    "Size(1366, 768)",
    "Size(1440, 900)",
    "textScale: size.width <= 390 ? 1.3 : 1",
]
for token in viewport_tokens:
    if token not in responsive_test:
        errors.append(f"START-23 viewport matrix missing: {token}")

critical_hu_literals = [
    "Create role",
    "Add administrator",
    "Commercial agreement reference *",
    "Activation-fee invoice reference *",
    "Partner Website Adapter",
    "Agreement status",
    "Backup storage",
    "Every successful restore point automatically queues a real restore test from the durable stored copy.",
    "Notifications",
    "Unread only",
    "Partner Portal",
    "Activate module",
    "Edit profile",
    "Add group",
    "Add relation",
    "Edit metrics",
]
for literal in critical_hu_literals:
    key = "'" + literal.replace("'", "\\'") + "':"
    if key not in localization:
        errors.append(f"Hungarian literal coverage missing: {literal}")

if "_start23Hu[value]" not in localization:
    errors.append("START-23 Hungarian literal map is not wired into HimateI18n.literal")

if errors:
    raise SystemExit("START-23 frontend audit failed:\n- " + "\n- ".join(errors))

print(
    "START-23 frontend audit passed: responsive primitives, viewport matrix, "
    "record-card policy, long-data protections and critical HU/EN coverage are present."
)
