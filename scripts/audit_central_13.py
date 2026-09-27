#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def check(ok: bool, message: str) -> None:
    if not ok:
        raise SystemExit("CENTRAL-13 FAIL: " + message)

frontend = read("frontend/lib/main.dart")
cms_ui = read("frontend/lib/cms_page.dart")
design_ui = read("frontend/lib/design_guide.dart")
connections_ui = read("frontend/lib/partner_connections.dart")
localization = read("frontend/lib/localization.dart")
cms_design = read("services/cmd/cms/design.go")
gateway = read("services/cmd/gateway/main.go")
gateway13 = read("services/cmd/gateway/central13.go")
connector = read("services/cmd/connector/central13.go")
connector_main = read("services/cmd/connector/main.go")
openapi = read("docs/openapi.yaml")
acceptance = read("docs/CENTRAL-13_ACCEPTANCE.md")

# Design Guide controls and backend validation must agree.
for token in [
    "'Primary Color'", "'Brand Color'", "'Page Background'", "'Body Text Color'",
    "Future<void> pickColor(", "colorHex(", "'Choose color'", "'Use color'",
    "static const headingFonts", "static const bodyFonts",
    "'Palatino'", "'Garamond'", "'Times New Roman'", "'Verdana'", "'Trebuchet MS'",
]:
    check(token in design_ui, f"Design Guide control missing: {token}")

for token in [
    '"Palatino":', '"Garamond":', '"Times New Roman":', '"Helvetica":',
    '"Verdana":', '"Trebuchet MS":', '"Courier New":',
]:
    check(token in cms_design, f"CMS font validation missing: {token}")

for token in [
    'case "Palatino":', 'case "Garamond":', 'case "Times New Roman":',
    'case "Helvetica":', 'case "Verdana":', 'case "Trebuchet MS":',
    "func designLayoutCSS(layout string) string",
    'case "modern_grid":', 'case "minimal":',
    'data-layout=%q',
]:
    check(token in gateway, f"real website typography/layout renderer missing: {token}")

# Workflow must be backed by CMS preview-token state and real publish.
for token in [
    "func (a *app) designPreviewActive() bool",
    'payload["preview_active"] = a.designPreviewActive()',
]:
    check(token in cms_design, f"CMS preview workflow state missing: {token}")
for token in [
    "bool previewActive = false;",
    "previewActive = response['preview_active'] == true;",
    "setState(() => previewActive = true);",
    "'Draft'", "'Preview'", "'Publish'", "'Active Brand'",
    "'Publish Active Brand'",
]:
    check(token in design_ui, f"Design workflow UI missing: {token}")

# CENTRAL-16 supersedes the old three-group Website hub with the approved
# six-card control center while preserving the same real underlying workspaces.
for token in [
    "Widget hubOverview()",
    "title: 'Design Guide'",
    "title: 'CMS'",
    "title: 'SEO'",
    "title: 'Domain & Deployment'",
    "title: 'Analytics'",
    "title: 'Partner Connections'",
    "setState(() => section = 'design')",
    "setState(() => section = 'pages')",
    "setState(() => section = 'seo')",
    "setState(() => section = 'domains')",
    "setState(() => section = 'analytics')",
    "setState(() => section = 'connections')",
    "label: const LText('Back')",
]:
    check(token in cms_ui, f"Website/Marketing hub contract missing: {token}")

# Partner Connections is partner-first and runtime-backed.
for token in [
    'privateMux.HandleFunc("/internal/v1/partner-connections",a.central13PartnerConnections)',
]:
    check(token in connector_main, f"Connector aggregate route missing: {token}")
for token in [
    "connector.credentials", "connector.partner_state", "connector.website_adapters",
    '"connection_status"', '"last_successful_sync"', '"last_error"',
    '"integration_count"', '"connection_types"', '"integrations"',
]:
    check(token in connector, f"Connector runtime aggregate missing: {token}")
for token in [
    "func (a *app) materializeCentralConnections(", "central13AllPartners",
    '"/internal/v1/partner-connections"', '"partner_name"',
    "central13ConnectionStatusForPartner", '"DELETED"',
    "func (a *app) central13Connections(",
    "centralSnapshotForRead(r.Context(), centralStep4ConnectionsKey)",
    '"source": "PERSISTED_CONNECTIONS_SCREEN"',
]:
    check(token in gateway13, f"Gateway Connections CQRS read model missing: {token}")
for token in [
    'path == "/api/v1/central/connections"', 'return "connectors"',
    'a.central13Connections(w, r, u)',
]:
    check(token in gateway, f"Connections RBAC/route missing: {token}")
for token in [
    "class PartnerConnectionsPanel", "'Partner Data Connections'",
    "'Last Successful Sync'", "'Last Error'", "'Connection Type'",
    "'ACTIVE'", "'INACTIVE'", "'SUSPENDED'", "'DELETED'",
    "centralConnectionsInitialPath()",
]:
    check(token in connections_ui or token in frontend, f"Connections UI missing: {token}")

# Vendor names cannot become top-level navigation/structure.
check("klaviyo" not in cms_ui.lower(), "Klaviyo leaked into top-level Website/Marketing structure")
check("klaviyo" not in connections_ui.lower(), "Klaviyo is hard-coded into the partner Connections UI")

# Canonical path/cache invalidation/RBAC remain, but CENTRAL-21 forbids
# hidden browser prewarm on hard refresh.
for token in [
    "String centralConnectionsInitialPath()",
    "add('/api/v1/central/connections')",
    "canConnections: can('connectors.read')",
]:
    check(token in frontend, f"Connections path/RBAC contract missing: {token}")
warm_start = frontend.find("void _warmControlPlane()")
warm_end = frontend.find("Future<void> _loadPublishedBrandAssets", warm_start)
warm = frontend[warm_start:warm_end] if warm_start >= 0 and warm_end > warm_start else ""
check("centralConnectionsInitialPath()" not in warm and "api.prefetch(" not in warm,
      "Connections still participates in browser hard-refresh prewarm")
check('strings.Contains(lowerKey, "/api/v1/central/connections?")' in read("services/cmd/gateway/central10.go"),
      "Connections server cache invalidation missing")

check("  /api/v1/central/connections:" in openapi, "OpenAPI Connections path missing")
check("CENTRAL-13" in acceptance and "smoke_central_13.sh" in acceptance,
      "CENTRAL-13 acceptance document incomplete")

# New explanatory Hungarian text must not silently fall back to English.
for token in [
    "'Primary Color': 'Elsődleges szín'",
    "'Brand Color': 'Márkaszín'",
    "'Partner Data Connections': 'Partner adatkapcsolatok'",
    "'Last Successful Sync': 'Utolsó sikeres szinkron'",
]:
    check(token in localization, f"Hungarian CENTRAL-13 localization missing: {token}")

print("CENTRAL-13 static acceptance passed")
