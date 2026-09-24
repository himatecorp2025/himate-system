#!/usr/bin/env sh
set -eu

BASE_URL="http://127.0.0.1:8080"
if [ "$#" -ge 1 ]; then BASE_URL="$1"; fi
EXPECTED_VERSION="$(printenv HIMATE_APP_VERSION 2>/dev/null || true)"
if [ -z "$EXPECTED_VERSION" ]; then EXPECTED_VERSION="0.8.31-start-23.11.6"; fi
TMP_ROOT="$(printenv TMPDIR 2>/dev/null || true)"
if [ -z "$TMP_ROOT" ]; then TMP_ROOT="/tmp"; fi
ADMIN_COOKIE="$TMP_ROOT/himate-start23116-admin.txt"
OWNER_A_COOKIE="$TMP_ROOT/himate-start23116-owner-a.txt"
OWNER_B_COOKIE="$TMP_ROOT/himate-start23116-owner-b.txt"
VIEWER_COOKIE="$TMP_ROOT/himate-start23116-viewer.txt"
BODY="$TMP_ROOT/himate-start23116-body.json"
rm -f "$ADMIN_COOKIE" "$OWNER_A_COOKIE" "$OWNER_B_COOKIE" "$VIEWER_COOKIE" "$BODY"
trap 'rm -f "$ADMIN_COOKIE" "$OWNER_A_COOKIE" "$OWNER_B_COOKIE" "$VIEWER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
A_EMAIL="notify-owner-a-$STAMP@himate.test"
B_EMAIL="notify-owner-b-$STAMP@himate.test"
V_EMAIL="notify-viewer-$STAMP@himate.test"
A_PASSWORD="NotifyA!12345Pass"
B_PASSWORD="NotifyB!12345Pass"
V_PASSWORD="NotifyV!12345Pass"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

admin_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$admin_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf '23.11.6 release is synchronized... '
HEALTH="$(curl -fsS "$BASE_URL/api/v1/health")"
python3 - "$HEALTH" "$EXPECTED_VERSION" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); expected=sys.argv[2]
assert d["status"]=="ok" and d["release_consistent"] is True,d
assert d["version"]==expected,(d.get("version"),expected)
PY
echo ok

create_partner() {
  email="$1"; suffix="$2"
  payload="$(python3 - "$STAMP" "$email" "$suffix" <<'PY'
import json,sys
stamp,email,suffix=sys.argv[1:]
print(json.dumps({
 "display_name":"START 23.11.6 Notifications "+suffix+" "+stamp,
 "legal_name":"START 23.11.6 Notifications "+suffix+" LLC",
 "brand_name":"Notifications "+suffix,
 "category_id":"cat_006","lifecycle":"PROSPECT",
 "contact_name":"Notification Owner "+suffix,"contact_email":email,
 "country":"United States","state_region":"New York","city":"New York",
 "onboarding_request_id":"start-23-11-6-"+suffix.lower()+"-"+stamp
}))
PY
)"
  curl -fsS -b "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/partners"
}

create_owner() {
  partner="$1"; email="$2"; password="$3"; name="$4"
  payload="$(python3 - "$email" "$password" "$name" <<'PY'
import json,sys
print(json.dumps({"name":sys.argv[3],"email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
  curl -fsS -b "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/partners/$partner/portal-users"
}

partner_login() {
  cookie="$1"; email="$2"; password="$3"
  payload="$(python3 - "$email" "$password" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
}

printf 'create two isolated Golden Test tenants and identities... '
A="$(create_partner "$A_EMAIL" A)"
B="$(create_partner "$B_EMAIL" B)"
A_ID="$(printf '%s' "$A" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
B_ID="$(printf '%s' "$B" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$ADMIN_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"START-23.11.6 notification module-scope proof"}' "$BASE_URL/api/v1/partners/$A_ID" >/dev/null
curl -fsS -b "$ADMIN_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"START-23.11.6 notification tenant proof"}' "$BASE_URL/api/v1/partners/$B_ID" >/dev/null
OWNER_A="$(create_owner "$A_ID" "$A_EMAIL" "$A_PASSWORD" "Notification Owner A")"
OWNER_B="$(create_owner "$B_ID" "$B_EMAIL" "$B_PASSWORD" "Notification Owner B")"
OWNER_A_ID="$(printf '%s' "$OWNER_A" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
OWNER_B_ID="$(printf '%s' "$OWNER_B" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
partner_login "$OWNER_A_COOKIE" "$A_EMAIL" "$A_PASSWORD"
partner_login "$OWNER_B_COOKIE" "$B_EMAIL" "$B_PASSWORD"
echo ok

printf 'owner creates viewer and restricts effective module access to Finance... '
VIEWER_PAYLOAD="$(python3 - "$V_EMAIL" "$V_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Notification Viewer","email":sys.argv[1],"password":sys.argv[2],"role":"viewer"}))
PY
)"
VIEWER="$(curl -fsS -b "$OWNER_A_COOKIE" -H 'Content-Type: application/json' -d "$VIEWER_PAYLOAD" "$BASE_URL/partner/api/v1/users")"
VIEWER_ID="$(printf '%s' "$VIEWER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$OWNER_A_COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"access_mode":"SELECTED","module_keys":["finance"]}' "$BASE_URL/partner/api/v1/users/$VIEWER_ID/modules" >/dev/null
partner_login "$VIEWER_COOKIE" "$V_EMAIL" "$V_PASSWORD"
sleep 1
echo ok

printf 'successful logins generated user-targeted Security notifications... '
OWNER_SECURITY_ID="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT id FROM notifications.events WHERE delivery_scope='PARTNER' AND partner_id='$A_ID' AND target_user_id='$OWNER_A_ID' AND event_type='SECURITY_LOGIN_DETECTED' ORDER BY id DESC LIMIT 1;")"
VIEWER_SECURITY_ID="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT id FROM notifications.events WHERE delivery_scope='PARTNER' AND partner_id='$A_ID' AND target_user_id='$VIEWER_ID' AND event_type='SECURITY_LOGIN_DETECTED' ORDER BY id DESC LIMIT 1;")"
test -n "$OWNER_SECURITY_ID"
test -n "$VIEWER_SECURITY_ID"
OWNER_FEED="$(curl -fsS -b "$OWNER_A_COOKIE" "$BASE_URL/partner/api/v1/notifications?limit=100")"
VIEWER_FEED="$(curl -fsS -b "$VIEWER_COOKIE" "$BASE_URL/partner/api/v1/notifications?limit=100")"
python3 - "$OWNER_FEED" "$OWNER_SECURITY_ID" "$VIEWER_SECURITY_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); ids={str(x["id"]) for x in d["items"]}
assert sys.argv[2] in ids,d
assert sys.argv[3] not in ids,d
assert d["delivery_scope"]=="PARTNER",d
PY
python3 - "$VIEWER_FEED" "$VIEWER_SECURITY_ID" "$OWNER_SECURITY_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); ids={str(x["id"]) for x in d["items"]}
assert sys.argv[2] in ids,d
assert sys.argv[3] not in ids,d
PY
echo ok

printf 'inject canonical central-engine events for delivery routing proof... '
WORKFLOW_FINANCE_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('TASK_ASSIGNED','INFO','New task assigned','A new Finance task was assigned.','workflow','$A_ID','/partner/app','modules.read','PARTNER','','finance','WORKFLOW') RETURNING id;")"
WORKFLOW_WORKSHOP_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('TASK_ASSIGNED','INFO','Workshop task assigned','A new Workshop task was assigned.','workflow','$A_ID','/partner/app','modules.read','PARTNER','','workshop_workflow','WORKFLOW') RETURNING id;")"
COMMENT_VIEWER_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('COMMENT_ADDED','INFO','New comment','Anna added a comment.','comments','$A_ID','/partner/app','modules.read','PARTNER','$VIEWER_ID','finance','COMMENT') RETURNING id;")"
DEADLINE_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('DEADLINE_TOMORROW','WARNING','Deadline tomorrow','A Finance deadline expires tomorrow.','workflow','$A_ID','/partner/app','modules.read','PARTNER','','finance','DEADLINE') RETURNING id;")"
CALENDAR_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('CALENDAR_EVENT_CREATED','INFO','New calendar event','A new Workshop event was added to the calendar.','calendar','$A_ID','/partner/app','modules.read','PARTNER','','workshop_workflow','CALENDAR') RETURNING id;")"
PAYMENT_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('PAYMENT_FAILED','WARNING','Automatic payment failed','The automatic payment failed.','billing','$A_ID','/partner/app','billing.read','PARTNER','','','BILLING') RETURNING id;")"
RETRY_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('PAYMENT_RETRY_SCHEDULED','WARNING','Payment retry scheduled','The next automatic charge is scheduled.','billing','$A_ID','/partner/app','billing.read','PARTNER','','','BILLING') RETURNING id;")"
SUSPEND_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('PARTNER_SUSPENDED_NONPAYMENT','CRITICAL','Service suspended','Service was suspended for non-payment.','billing','$A_ID','/partner/app','billing.read','PARTNER','','','BILLING') RETURNING id;")"
ADMIN_ONLY_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('USER_ACCESS_REVIEW','INFO','User access review','Review organization user access.','users','$A_ID','/partner/app','users.write','PARTNER','','','SYSTEM') RETURNING id;")"
TENANT_B_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('TASK_ASSIGNED','INFO','Tenant B task','Only tenant B may see this.','workflow','$B_ID','/partner/app','modules.read','PARTNER','','finance','WORKFLOW') RETURNING id;")"
PLATFORM_ID="$(docker compose exec -T postgres psql -U himate -d himate -A -t -q -c "INSERT INTO notifications.events(event_type,severity,title,message,resource,partner_id,deep_link,audience_permission,delivery_scope,target_user_id,module_key,category) VALUES('PLATFORM_TEST_EVENT','INFO','Platform event','Partner Portal must not receive this.','system','$A_ID','/app','','PLATFORM','','','SYSTEM') RETURNING id;")"
echo ok

printf 'viewer feed applies tenant + target + permission + SELECTED module intersection... '
VIEWER_FEED="$(curl -fsS -b "$VIEWER_COOKIE" "$BASE_URL/partner/api/v1/notifications?limit=100")"
python3 - "$VIEWER_FEED" "$WORKFLOW_FINANCE_ID" "$WORKFLOW_WORKSHOP_ID" "$COMMENT_VIEWER_ID" "$DEADLINE_ID" "$CALENDAR_ID" "$PAYMENT_ID" "$RETRY_ID" "$SUSPEND_ID" "$ADMIN_ONLY_ID" "$TENANT_B_ID" "$PLATFORM_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); ids={str(x["id"]) for x in d["items"]}
wf_fin,wf_work,comment,deadline,calendar,payment,retry,suspend,admin_only,tenant_b,platform=sys.argv[2:]
for expected in [wf_fin,comment,deadline,payment,retry,suspend]:
    assert expected in ids,(expected,ids)
for hidden in [wf_work,calendar,admin_only,tenant_b,platform]:
    assert hidden not in ids,(hidden,ids)
cats={x["category"] for x in d["items"]}
assert {"WORKFLOW","COMMENT","DEADLINE","BILLING","SECURITY"}.issubset(cats),cats
PY
echo ok

printf 'owner feed sees own-tenant role/module events but not viewer-targeted or foreign scope... '
OWNER_FEED="$(curl -fsS -b "$OWNER_A_COOKIE" "$BASE_URL/partner/api/v1/notifications?limit=100")"
python3 - "$OWNER_FEED" "$WORKFLOW_FINANCE_ID" "$WORKFLOW_WORKSHOP_ID" "$COMMENT_VIEWER_ID" "$DEADLINE_ID" "$CALENDAR_ID" "$PAYMENT_ID" "$ADMIN_ONLY_ID" "$TENANT_B_ID" "$PLATFORM_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); ids={str(x["id"]) for x in d["items"]}
wf_fin,wf_work,comment,deadline,calendar,payment,admin_only,tenant_b,platform=sys.argv[2:]
for expected in [wf_fin,wf_work,deadline,calendar,payment,admin_only]:
    assert expected in ids,(expected,ids)
for hidden in [comment,tenant_b,platform]:
    assert hidden not in ids,(hidden,ids)
PY
echo ok

printf 'tenant B cannot read tenant A events... '
B_FEED="$(curl -fsS -b "$OWNER_B_COOKIE" "$BASE_URL/partner/api/v1/notifications?limit=100")"
python3 - "$B_FEED" "$TENANT_B_ID" "$WORKFLOW_FINANCE_ID" "$PAYMENT_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); ids={str(x["id"]) for x in d["items"]}
assert sys.argv[2] in ids,d
assert sys.argv[3] not in ids,d
assert sys.argv[4] not in ids,d
PY
echo ok

printf 'platform feed remains separate from PARTNER delivery... '
PLATFORM_FEED="$(curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/notifications?limit=100")"
python3 - "$PLATFORM_FEED" "$PLATFORM_ID" "$WORKFLOW_FINANCE_ID" "$PAYMENT_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); ids={str(x["id"]) for x in d["items"]}
assert sys.argv[2] in ids,d
assert sys.argv[3] not in ids,d
assert sys.argv[4] not in ids,d
assert d["delivery_scope"]=="PLATFORM",d
PY
echo ok

printf 'hidden notifications cannot be marked read by the viewer... '
CODE="$(status "$VIEWER_COOKIE" POST "/partner/api/v1/notifications/$WORKFLOW_WORKSHOP_ID/read")"
test "$CODE" = "403"
CODE="$(status "$VIEWER_COOKIE" POST "/partner/api/v1/notifications/$ADMIN_ONLY_ID/read")"
test "$CODE" = "403"
CODE="$(status "$VIEWER_COOKIE" POST "/partner/api/v1/notifications/$OWNER_SECURITY_ID/read")"
test "$CODE" = "403"
CODE="$(status "$VIEWER_COOKIE" POST "/partner/api/v1/notifications/$TENANT_B_ID/read")"
test "$CODE" = "404"
echo ok

printf 'visible read and read-all persist only the viewer-visible event set... '
BEFORE="$(curl -fsS -b "$VIEWER_COOKIE" "$BASE_URL/partner/api/v1/notifications?limit=100")"
BEFORE_UNREAD="$(printf '%s' "$BEFORE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["unread_count"])')"
curl -fsS -b "$VIEWER_COOKIE" -X POST "$BASE_URL/partner/api/v1/notifications/$WORKFLOW_FINANCE_ID/read" >/dev/null
AFTER_ONE="$(curl -fsS -b "$VIEWER_COOKIE" "$BASE_URL/partner/api/v1/notifications?limit=100")"
AFTER_ONE_UNREAD="$(printf '%s' "$AFTER_ONE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["unread_count"])')"
test "$AFTER_ONE_UNREAD" -lt "$BEFORE_UNREAD"
curl -fsS -b "$VIEWER_COOKIE" -X POST "$BASE_URL/partner/api/v1/notifications/read-all" >/dev/null
AFTER_ALL="$(curl -fsS -b "$VIEWER_COOKIE" "$BASE_URL/partner/api/v1/notifications?limit=100")"
python3 - "$AFTER_ALL" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); assert d["unread_count"]==0,d
PY
HIDDEN_READS="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM notifications.read_state WHERE user_id='$VIEWER_ID' AND event_id IN ($WORKFLOW_WORKSHOP_ID,$CALENDAR_ID,$ADMIN_ONLY_ID,$OWNER_SECURITY_ID,$TENANT_B_ID,$PLATFORM_ID);")"
test "$HIDDEN_READS" = "0"
echo ok

printf 'notification read mutations are gateway-audited... '
sleep 1
AUDIT="$(curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/audit/events?partner_id=$A_ID&limit=100")"
python3 - "$AUDIT" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); actions={x["action"] for x in d["items"]}
assert "PARTNER_NOTIFICATION_READ" in actions,actions
assert "PARTNER_NOTIFICATIONS_READ_ALL" in actions,actions
PY
echo ok

echo 'HIMATE START-23.11.6 Central Notifications smoke passed'
