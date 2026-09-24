#!/usr/bin/env python3
from pathlib import Path
import json
import sys

root=Path(__file__).resolve().parents[1]
notifications=(root/"services/cmd/notifications/main.go").read_text()
notification_tests=(root/"services/cmd/notifications/main_test.go").read_text()
gateway=(root/"services/cmd/gateway/partner_portal.go").read_text()
partner_notifications=(root/"services/cmd/gateway/partner_notifications.go").read_text()
gateway_main=(root/"services/cmd/gateway/main.go").read_text()
billing=(root/"services/cmd/billing/dunning.go").read_text()
panel=(root/"frontend/lib/notifications_panel.dart").read_text()
portal=(root/"frontend/lib/partner_portal.dart").read_text()
openapi=(root/"docs/openapi.yaml").read_text()
render=(root/"render.yaml").read_text()
compose=(root/"docker-compose.yml").read_text()
fast=(root/".github/workflows/ci-fast.yml").read_text()
full=(root/".github/workflows/ci.yml").read_text()
acceptance=(root/"docs/START-23.11.6_ACCEPTANCE.md").read_text()
matrix=json.loads((root/"docs/START-23.1_FUNCTIONAL_MATRIX.json").read_text())
release="0.8.31-start-23.11.6"

checks=[
 ("central notification migration persists delivery scope, target user, module and category",
  'Version: 2' in notifications and 'start-23-11-6-central-delivery-engine' in notifications and
  'delivery_scope' in notifications and 'target_user_id' in notifications and 'module_key' in notifications and 'category' in notifications),
 ("canonical categories cover roadmap event classes",
  all(token in notifications for token in ['"WORKFLOW"','"COMMENT"','"DEADLINE"','"CALENDAR"','"BILLING"','"SECURITY"','"SYSTEM"'])),
 ("Partner delivery is tenant scoped",
  'strings.TrimSpace(event.PartnerID) != strings.TrimSpace(partnerID)' in notifications and
  'PARTNER_SCOPE_REQUIRED' in notifications),
 ("target-user delivery is enforced",
  'event.TargetUserID != "" && event.TargetUserID != userID' in notifications),
 ("role permission delivery is enforced",
  'permissionVisible(event.AudiencePermission, permissions)' in notifications),
 ("START-23.11.5 module access is part of notification delivery",
  'event.ModuleKey != "" && !modules[event.ModuleKey] && !modules["*"]' in notifications and
  'X-Himate-Module-Keys' in partner_notifications and 'loadPartnerUserModulePolicy' in partner_notifications),
 ("read and read-all re-evaluate event visibility",
  notifications.count('eventVisible(event, scope, partnerID, userID, permissions, modules)')>=2 and
  'Notification is outside your delivery scope' in notifications),
 ("Partner roles expose notifications.read",
  gateway.count('"notifications.read"')>=3 and 'a.partnerNotifications(w,r,u)' in gateway),
 ("Partner gateway sends explicit partner delivery headers",
  '"X-Himate-Notification-Scope": "PARTNER"' in partner_notifications and
  '"X-Himate-Partner-ID"' in partner_notifications and '"X-Himate-Permissions"' in partner_notifications),
 ("successful Partner Portal login emits targeted Security event without blocking login",
  'go a.emitPartnerLoginNotification(u)' in gateway and
  '"SECURITY_LOGIN_DETECTED"' in partner_notifications and '"target_user_id"' in partner_notifications),
 ("Billing dunning publishes through central PARTNER Billing category",
  '"delivery_scope": "PARTNER"' in billing and '"category": "BILLING"' in billing and
  '/internal/v1/notifications/events' in billing),
 ("control-plane events remain explicitly PLATFORM scoped",
  '"delivery_scope":"PLATFORM"' in gateway_main),
 ("shared Flutter notification center supports endpoint prefixes",
  "this.endpointPrefix = '/api/v1/notifications'" in panel and
  "widget.endpointPrefix" in panel and "/read-all" in panel),
 ("Partner Portal renders central notification bell in desktop and responsive headers",
  portal.count("endpointPrefix: '/partner/api/v1/notifications'")>=2 and
  portal.count("can('notifications.read')")>=2),
 ("notification unit tests cover category and delivery isolation",
  'TestNotificationCategoryClassification' in notification_tests and
  'TestPartnerNotificationVisibility' in notification_tests),
 ("OpenAPI publishes Partner Portal notification feed and read mutations",
  '/partner/api/v1/notifications:' in openapi and
  '/partner/api/v1/notifications/read-all:' in openapi and
  '/partner/api/v1/notifications/{notificationId}/read:' in openapi and
  'version: '+release in openapi and 'START-01 through START-23.11.6' in openapi),
 ("release aligned in Render and Compose",
  render.count("value: "+release)==19 and
  compose.count("HIMATE_APP_VERSION: ${HIMATE_APP_VERSION:-"+release+"}")==18),
 ("Fast and full CI enforce START-23.11.6",
  fast.count("audit_start_23_11_6.py")>=2 and "smoke_start_23_11_6.sh" in fast and
  "audit_start_23_11_6.py" in full and "smoke_start_23_11_6.sh" in full),
 ("acceptance freezes one-engine delivery pipeline",
  "event → delivery scope → tenant → target user → role permission → effective user-module access → Partner Portal inbox" in acceptance),
]
ids={str(x.get("id")) for x in matrix.get("contracts",[])}
checks.append(("functional matrix contains Partner notification read", "PORTAL-NOTIFY-READ-23-11-6" in ids))
checks.append(("functional matrix contains Partner notification read-all", "PORTAL-NOTIFY-READALL-23-11-6" in ids))
checks.append(("functional matrix closes through 23.11.6", matrix.get("completed_through")=="23.11.6"))

failures=[label for label,ok in checks if not ok]
if failures:
    for failure in failures:
        print("FAIL:",failure)
    sys.exit(1)
print("START-23.11.6 Central Notifications static audit: PASS")
