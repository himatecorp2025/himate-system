#!/usr/bin/env python3
from pathlib import Path
import json
import sys

root=Path(__file__).resolve().parents[1]
gateway=(root/"services/cmd/gateway/partner_portal.go").read_text()
access=(root/"services/cmd/gateway/partner_user_modules.go").read_text()
main=(root/"services/cmd/gateway/main.go").read_text()
portal=(root/"frontend/lib/partner_portal.dart").read_text()
openapi=(root/"docs/openapi.yaml").read_text()
render=(root/"render.yaml").read_text()
compose=(root/"docker-compose.yml").read_text()
fast=(root/".github/workflows/ci-fast.yml").read_text()
full=(root/".github/workflows/ci.yml").read_text()
acceptance=(root/"docs/START-23.11.5_ACCEPTANCE.md").read_text()
matrix=json.loads((root/"docs/START-23.1_FUNCTIONAL_MATRIX.json").read_text())
release="0.8.31-start-23.11.6"

checks=[
 ("identity migration is registered after historical migrations",
  "partnerUserModulePermissionsMigration()" in main and "Version: 13" in access and
  "module_access_mode" in access and "identity.partner_user_modules" in access),
 ("backward-compatible ALL_OWNED and explicit SELECTED modes exist",
  'partnerModuleAccessAllOwned = "ALL_OWNED"' in access and
  'partnerModuleAccessSelected = "SELECTED"' in access and
  "DEFAULT 'ALL_OWNED'" in access),
 ("effective access is partner entitlement intersect user assignment",
  "partnerOwnedModuleMap" in access and '"ACTIVE"' in access and 'item["executable"] == true' in access and
  "mode == partnerModuleAccessAllOwned || selected[key]" in access),
 ("not-owned assignments are rejected",
  '"MODULE_NOT_OWNED"' in access and "not ACTIVE and executable for the partner" in access),
 ("Marketplace stays visible but exposes user access state",
  '"GRANTED"' in access and '"NOT_ASSIGNED"' in access and '"ORGANIZATION_LOCKED"' in access and
  'item["user_executable"] = granted' in access),
 ("Partner Portal module responses apply the user policy",
  "applyPartnerUserModuleAccess" in gateway and gateway.count("applyPartnerUserModuleAccess")>=2),
 ("own-tenant user module GET/PUT route is permission protected",
  'strings.HasSuffix(path,"/modules")' in gateway and '"users.read"' in gateway and '"users.write"' in gateway and
  "partnerUserModuleAccess(w,r,u,raw)" in gateway),
 ("owner boundary is enforced",
  'actor.Role != "owner" && targetRole == "owner"' in access),
 ("Portal UI exposes explicit module-access editor",
  "Future<void> manageUserModuleAccess(" in portal and
  "All partner-owned modules" in portal and "Selected modules only" in portal and
  "widget.api.put('/partner/api/v1/users/$userId/modules'" in portal),
 ("current user cards distinguish plan entitlement from user assignment",
  "user_access_state" in portal and "user_executable" in portal and "NOT ASSIGNED" in portal and
  "Your organization owns this module, but it is not assigned to your user account." in portal),
 ("default module only auto-opens when current user has effective access",
  "module['user_executable'] == true" in portal),
 ("OpenAPI publishes user-module permission contract",
  "/partner/api/v1/users/{userId}/modules:" in openapi and "version: "+release in openapi and
  "START-01 through START-23.11.6" in openapi),
 ("release aligned in Render and Compose",
  render.count("value: "+release)==19 and
  compose.count("HIMATE_APP_VERSION: ${HIMATE_APP_VERSION:-"+release+"}")==18),
 ("Fast and full CI enforce START-23.11.5",
  fast.count("audit_start_23_11_5.py")>=2 and "smoke_start_23_11_5.sh" in fast and
  "audit_start_23_11_5.py" in full and "smoke_start_23_11_5.sh" in full),
 ("acceptance freezes entitlement intersection",
  "partner ACTIVE + executable entitlement ∩ user assignment" in acceptance),
]
ids={str(x.get("id")) for x in matrix.get("contracts",[])}
checks.append(("functional matrix contains PORTAL-USER-MODULE-ACCESS-23-11-5",
               "PORTAL-USER-MODULE-ACCESS-23-11-5" in ids))
completed_through=str(matrix.get("completed_through",""))
checks.append((
    "functional matrix includes START-23.11.5 closure",
    completed_through in {"23.11.5","23.11.6","23.11.7","23.12"},
))

failures=[label for label,ok in checks if not ok]
if failures:
    for failure in failures: print("FAIL:",failure)
    sys.exit(1)
print("START-23.11.5 User ↔ Module Permissions static audit: PASS")
