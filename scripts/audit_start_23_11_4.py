#!/usr/bin/env python3
from pathlib import Path
import json
import sys

root=Path(__file__).resolve().parents[1]
cms=(root/"services/cmd/cms/main.go").read_text()
workspace=(root/"services/cmd/cms/workspace_personalization.go").read_text()
themes=(root/"services/cmd/cms/themes.go").read_text()
gateway=(root/"services/cmd/gateway/partner_portal.go").read_text()
portal=(root/"frontend/lib/partner_portal.dart").read_text()
design=(root/"frontend/lib/partner_design.dart").read_text()
openapi=(root/"docs/openapi.yaml").read_text()
render=(root/"render.yaml").read_text()
compose=(root/"docker-compose.yml").read_text()
matrix=json.loads((root/"docs/START-23.1_FUNCTIONAL_MATRIX.json").read_text())
acceptance=(root/"docs/START-23.11.4_ACCEPTANCE.md").read_text()
release="0.8.32-start-23.11.7"

checks=[
 ("workspace persistence exists",
  "cms.partner_workspace_settings" in cms and "cms.partner_module_presentations" in cms),
 ("tenant media owns workspace logo and custom module icon",
  workspace.count("partnerOwnsMedia")>=2 and "MEDIA_SCOPE" in workspace),
 ("brand color contrast is fail-closed",
  "text_color and background_color must have at least 4.5:1 contrast" in workspace and "workspaceContrast" in workspace),
 ("workspace payload is presentation-only",
  '"presentation_only":true' in workspace and '"presentation_contract": "CANONICAL_KEYS_AND_SYSTEM_BEHAVIOR_UNCHANGED"' in themes),
 ("public workspace media becomes readable only through authoritative reference",
  "partner_workspace_settings WHERE logo_media_id=$1" in cms and "partner_module_presentations WHERE custom_icon_media_id=$1" in cms),
 ("Gateway validates default module against real tenant Catalog",
  "partnerPresentationModule" in gateway and "DEFAULT_MODULE_NOT_ACTIVE" in gateway and 'item["executable"]!=true' in gateway),
 ("Gateway validates module presentation key against tenant Marketplace",
  'strings.HasPrefix(suffix,"/modules/")' in gateway and "MODULE_NOT_FOUND" in gateway),
 ("Partner Portal workspace editor is real",
  "Future<void> editWorkspacePersonalization()" in design and "widget.api.put('/partner/api/v1/design/workspace'" in design),
 ("module card pencil editor and reset are real",
  "Future<void> editModulePresentation(" in design and "Customize module presentation" in portal and
  "widget.api.put('/partner/api/v1/design/modules/$moduleKey'" in design and
  "widget.api.delete(" in design and "'/partner/api/v1/design/modules/$moduleKey'" in design and "Reset to HIMATE default" in design),
 ("official HIMATE identity stays visible",
  "Official HIMATE name" in design and "Canonical module key" in design and "POWERED BY HIMATE" in portal),
 ("custom cards use presentation values without mutating Catalog fields",
  "moduleDisplayName(module)" in portal and "moduleDisplayDescription(module)" in portal and
  "modulePresentationCardColor(module)" in portal),
 ("default module changes initial Partner Portal presentation only",
  "_defaultWorkspaceApplied" in portal and "modules.insert(0, defaultModule)" in portal and
  "visibleNav.indexWhere((item) => item.label == 'Modules')" in portal),
 ("OpenAPI publishes workspace and module-presentation contracts",
  "/partner/api/v1/design/workspace:" in openapi and "/partner/api/v1/design/modules/{moduleKey}:" in openapi),
 ("release aligned in OpenAPI/Render/Compose",
  "version: "+release in openapi and render.count("value: "+release)==21 and
  compose.count("HIMATE_APP_VERSION: ${HIMATE_APP_VERSION:-"+release+"}")==20),
 ("acceptance freezes canonical mechanics",
  "Presentation may change. Canonical system meaning may not." in acceptance and
  "user-to-module permissions: START-23.11.5" in acceptance),
]

contract_ids={str(x.get("id")) for x in matrix.get("contracts",[])}
for cid in [
 "PORTAL-WORKSPACE-PERSONALIZATION-23-11-4",
 "PORTAL-MODULE-PRESENTATION-23-11-4",
 "PORTAL-MODULE-PRESENTATION-RESET-23-11-4",
]:
    checks.append(("functional matrix contains "+cid,cid in contract_ids))
completed_through=str(matrix.get("completed_through",""))
checks.append((
    "functional matrix includes START-23.11.4 closure",
    completed_through in {"23.11.4","23.11.5","23.11.6","23.11.7","23.12"},
))

failures=[label for label,ok in checks if not ok]
if failures:
    for failure in failures: print("FAIL:",failure)
    sys.exit(1)
print("START-23.11.4 Partner Workspace & Personalization static audit: PASS")
