#!/usr/bin/env python3
from pathlib import Path
import json
import sys

root=Path(__file__).resolve().parents[1]
portal=(root/"frontend/lib/partner_portal.dart").read_text()
design=(root/"frontend/lib/partner_design.dart").read_text()
notifications_ui=(root/"frontend/lib/notifications_panel.dart").read_text()
gateway=(root/"services/cmd/gateway/partner_portal.go").read_text()
user_modules=(root/"services/cmd/gateway/partner_user_modules.go").read_text()
notifications=(root/"services/cmd/notifications/main.go").read_text()
cms=(root/"services/cmd/cms/main.go").read_text()
catalog=(root/"services/cmd/catalog/main.go").read_text()
billing_plans=(root/"services/cmd/billing/plans.go").read_text()
openapi=(root/"docs/openapi.yaml").read_text()
render=(root/"render.yaml").read_text()
compose=(root/"docker-compose.yml").read_text()
fast=(root/".github/workflows/ci-fast.yml").read_text()
full=(root/".github/workflows/ci.yml").read_text()
acceptance=(root/"docs/START-23.11.7_ACCEPTANCE.md").read_text()
matrix=json.loads((root/"docs/START-23.1_FUNCTIONAL_MATRIX.json").read_text())

release="0.8.32-start-23.11.7"

frontend_api_tokens=[
    "/partner/api/v1/dashboard",
    "/partner/api/v1/company",
    "/partner/api/v1/modules/",
    "/partner/api/v1/billing/subscriptions",
    "/partner/api/v1/billing/invoices",
    "/partner/api/v1/plans",
    "/partner/api/v1/plan",
    "/partner/api/v1/charity",
    "/partner/api/v1/charity/request",
    "/partner/api/v1/charity/modules",
    "/partner/api/v1/users",
    "/partner/api/v1/design",
    "/partner/api/v1/design/media",
    "/partner/api/v1/design/workspace",
    "/partner/api/v1/design/modules/",
]
openapi_paths=[
    "/partner/api/v1/auth/login:",
    "/partner/api/v1/auth/logout:",
    "/partner/api/v1/auth/me:",
    "/partner/api/v1/dashboard:",
    "/partner/api/v1/company:",
    "/partner/api/v1/modules:",
    "/partner/api/v1/modules/{moduleKey}/activate:",
    "/partner/api/v1/modules/{moduleKey}/subscription:",
    "/partner/api/v1/billing/summary:",
    "/partner/api/v1/plans:",
    "/partner/api/v1/plan:",
    "/partner/api/v1/plan/modules:",
    "/partner/api/v1/charity:",
    "/partner/api/v1/charity/request:",
    "/partner/api/v1/charity/modules:",
    "/partner/api/v1/billing/subscriptions:",
    "/partner/api/v1/billing/invoices:",
    "/partner/api/v1/impact/summary:",
    "/partner/api/v1/users:",
    "/partner/api/v1/users/{userId}:",
    "/partner/api/v1/users/{userId}/modules:",
    "/partner/api/v1/notifications:",
    "/partner/api/v1/notifications/read-all:",
    "/partner/api/v1/notifications/{notificationId}/read:",
    "/partner/api/v1/design:",
    "/partner/api/v1/design/media:",
    "/partner/api/v1/design/profiles:",
    "/partner/api/v1/design/profiles/{profileId}:",
    "/partner/api/v1/design/profiles/{profileId}/activate:",
    "/partner/api/v1/design/workspace:",
    "/partner/api/v1/design/modules/{moduleKey}:",
]

checks=[
 ("Partner Portal frontend API surfaces are present",
  all(token in portal+design for token in frontend_api_tokens) and
  "endpointPrefix: '/partner/api/v1/notifications'" in portal),
 ("Partner Portal gateway routes every principal frontend domain",
  all(token in gateway for token in [
    'case path=="/dashboard"',
    'case path=="/company"',
    'case path=="/modules"',
    'case path=="/users"',
    'strings.HasPrefix(path,"/notifications/")',
    'case path=="/design"',
    'case path=="/design/workspace"',
  ])),
 ("OpenAPI publishes the full Partner Portal route family",
  all(path in openapi for path in openapi_paths)),
 ("OpenAPI includes module-presentation reset DELETE",
  "/partner/api/v1/design/modules/{moduleKey}:" in openapi and
  "summary: Reset own-tenant module presentation to canonical HIMATE defaults" in openapi),
 ("initial Portal rendering no longer waits for secondary panels",
  "final extrasFuture = Future.wait<Map<String, dynamic>?>" in portal and
  "final extras = await extrasFuture;" in portal and
  portal.find("loading = false;") < portal.find("final extras = await extrasFuture;")),
 ("Marketplace plan context is fetched concurrently",
  "var wg sync.WaitGroup" in gateway and
  'plansErr=a.internalGET' in gateway and 'currentErr=a.internalGET' in gateway and 'wg.Wait()' in gateway),
 ("Partner user list removes per-user N+1 module metadata reads",
  "pu.module_access_mode" in gateway and
  "COALESCE((SELECT COUNT(*) FROM identity.partner_user_modules pum" in gateway),
 ("user-module policy overlaps identity and Catalog entitlement reads",
  '"sync"' in user_modules and
  "wg.Add(2)" in user_modules and
  "selectionErr" in user_modules and "ownedErr" in user_modules),
 ("notification query filters target-user events in SQL",
  "AND (e.target_user_id='' OR e.target_user_id=$1)" in notifications),
 ("notification read authorization preserves 403 for hidden same-tenant targets and 404 for out-of-scope IDs",
  "func (a *app) queryEventByIDInScope" in notifications and
  "err == sql.ErrNoRows" in notifications and
  'Notification is outside your delivery scope' in notifications),
 ("notification read-all uses one bulk UPSERT instead of one Exec per event",
  'VALUES ' in notifications and 'strings.Join(values, ",")' in notifications and
  'ON CONFLICT(event_id,user_id) DO UPDATE SET read_at=NOW()' in notifications),
 ("identity module assignment has a covering tenant/user/module index",
  "identity_partner_user_modules_partner_idx" in user_modules and
  "partner_id,user_id,module_key" in user_modules),
 ("notification delivery paths have partner/target/module indexes",
  "notifications_partner_delivery_idx" in notifications and
  "notifications_target_user_idx" in notifications and
  "notifications_module_delivery_idx" in notifications),
 ("workspace personalization tables are tenant keyed and indexed",
  "partner_id TEXT PRIMARY KEY" in cms and
  "PRIMARY KEY(partner_id,module_key)" in cms and
  "cms_partner_module_presentations_partner_idx" in cms),
 ("Catalog partner module access is tenant indexed",
  "partner_modules_status_idx" in catalog and
  "partner_modules_entitlement_idx" in catalog and
  "partner_modules_commercial_idx" in catalog),
 ("Billing partner plan reads and due-cycle scans are indexed",
  "billing_partner_plan_due_idx" in billing_plans and
  "partner_id TEXT PRIMARY KEY" in billing_plans and
  "billing_partner_plan_selection_effective_idx" in billing_plans),
 ("responsive Portal has explicit desktop/mobile shell boundary and adaptive grids",
  "final wide = constraints.maxWidth >= 900;" in portal and
  "drawer: Drawer(" in portal and
  "constraints.maxWidth < 650" in portal and
  "constraints.maxWidth < 760" in portal),
 ("shared notification UI is responsive",
  "MediaQuery.sizeOf(context).width < 620" in notifications_ui),
 ("23.11.7 release is aligned in OpenAPI, Render and Compose",
  "version: "+release in openapi and
  render.count("value: "+release)==20 and
  compose.count("HIMATE_APP_VERSION: ${HIMATE_APP_VERSION:-"+release+"}")==19),
 ("fast CI carries inherited closure gates and the 23.11.7 audit",
  all(("audit_start_23_11_"+x+".py") in fast for x in ["4","5","6","7"])),
 ("full CI carries inherited closure gates and the 23.11.7 audit/smoke",
  "audit_start_23_11_7.py" in full and "smoke_start_23_11_7.sh" in full),
 ("acceptance document records sectioned frontend/backend/database/API/responsive closure",
  all(token in acceptance for token in [
    "Frontend closure","Backend / API closure","Database / performance closure",
    "Responsive / UX closure","Regression / release closure"
  ])),
]

ids={str(x.get("id")) for x in matrix.get("contracts",[])}
checks.append(("functional matrix contains Partner Portal closure performance contract",
               "PORTAL-CLOSURE-PERFORMANCE-23-11-7" in ids))
checks.append(("functional matrix closes through 23.11.7",
               matrix.get("completed_through")=="23.11.7"))

failures=[label for label,ok in checks if not ok]
if failures:
    for failure in failures:
        print("FAIL:",failure)
    sys.exit(1)
print("START-23.11.7 Partner Portal Closure Audit: PASS")
