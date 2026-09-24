#!/usr/bin/env python3
from pathlib import Path
import json

ROOT = Path(__file__).resolve().parents[1]

portal = (ROOT / "frontend/lib/partner_portal.dart").read_text()
partner_gateway = (ROOT / "services/cmd/gateway/partner_portal.go").read_text()
user_modules = (ROOT / "services/cmd/gateway/partner_user_modules.go").read_text()
gateway = (ROOT / "services/cmd/gateway/main.go").read_text()
openapi = (ROOT / "docs/openapi.yaml").read_text()
matrix = json.loads((ROOT / "docs/START-23.1_FUNCTIONAL_MATRIX.json").read_text())
full_ci = (ROOT / ".github/workflows/ci.yml").read_text()

errors = []

def require(condition, message):
    if not condition:
        errors.append(message)

require("secondaryLoadErrors" in portal, "Partner Portal degraded secondary-load state is missing")
require("HIMATE kept the last successfully loaded values instead of showing missing data as empty" in portal,
        "Partner Portal does not explain degraded secondary-service state")
require("subscriptions = extras[0] == null ? <Map<String, dynamic>>[] : items(extras[0]!)" not in portal,
        "Partner Portal still collapses failed subscription loads to an empty list")
require("response['entitlement_sync_pending'] == true" in portal,
        "Partner Portal does not handle entitlement_sync_pending")
require("warning: true" in portal,
        "Pending entitlement synchronization is not surfaced as a warning")

require("func (a *app) requirePartnerModuleExecution" in user_modules,
        "Central partner module execution guard is missing")
require('"MODULE_NOT_OWNED"' in user_modules,
        "Organization entitlement denial is missing")
require('"MODULE_NOT_ASSIGNED"' in user_modules,
        "Per-user module assignment denial is missing")
require('"PARTNER_ENTITLEMENT_INTERSECT_USER_ASSIGNMENT"' in user_modules,
        "Effective module-access rule marker is missing")
require('strings.HasPrefix(path,"/runtime/modules/")' in partner_gateway,
        "Partner runtime module namespace is not routed through the guard")

require('"OWNER_REQUIRED", "Only the HIMATE system owner can activate Golden Test Partner mode"' in gateway,
        "Golden Test Partner activation is not restricted to system owner")
require('requested, present := payload["test_partner"]' in gateway,
        "Golden Test Partner mutation inspection is missing")

entry = next((x for x in matrix.get("contracts", []) if x.get("id") == "PORTAL-MODULE-ACTIVATE"), None)
require(entry is not None, "PORTAL-MODULE-ACTIVATE contract is missing")
if entry is not None:
    require(entry.get("current_state") != "PARTIAL_PRODUCT",
            "PORTAL-MODULE-ACTIVATE still has stale PARTIAL_PRODUCT state")
    require("smoke_start_23_12_phase1.sh" in str(entry.get("e2e_proof", "")),
            "PORTAL-MODULE-ACTIVATE does not point to Phase 1 runtime proof")

require("/partner/api/v1/runtime/modules/{moduleKey}/access:" in openapi,
        "Guarded runtime module access endpoint is missing from OpenAPI")
require("audit_start_23_12_phase1.py" in full_ci,
        "Full CI does not run START-23.12 Phase 1 source audit")
require("smoke_start_23_12_phase1.sh" in full_ci,
        "Full CI does not run START-23.12 Phase 1 runtime smoke")

if errors:
    raise SystemExit("START-23.12 Phase 1 audit failed:\n- " + "\n- ".join(errors))

print("START-23.12 Phase 1 source audit passed: degraded-state handling, entitlement convergence feedback, runtime module authorization and Golden Test privilege boundary are closed.")
