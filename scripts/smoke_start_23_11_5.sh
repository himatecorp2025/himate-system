#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
EXPECTED_VERSION="${HIMATE_APP_VERSION:-0.8.33-start-23.12}"
TMP_ROOT="${TMPDIR:-/tmp}"
ADMIN_COOKIE="$TMP_ROOT/himate-start23115-admin.txt"
OWNER_A_COOKIE="$TMP_ROOT/himate-start23115-owner-a.txt"
OWNER_B_COOKIE="$TMP_ROOT/himate-start23115-owner-b.txt"
VIEWER_COOKIE="$TMP_ROOT/himate-start23115-viewer.txt"
BODY="$TMP_ROOT/himate-start23115-body.json"
OWNER_C_COOKIE="$TMP_ROOT/himate-start23115-owner-c.txt"
rm -f "$ADMIN_COOKIE" "$OWNER_A_COOKIE" "$OWNER_B_COOKIE" "$VIEWER_COOKIE" "$OWNER_C_COOKIE" "$BODY"
trap 'rm -f "$ADMIN_COOKIE" "$OWNER_A_COOKIE" "$OWNER_B_COOKIE" "$VIEWER_COOKIE" "$OWNER_C_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
A_EMAIL="modules-owner-a-${STAMP}@himate.test"
B_EMAIL="modules-owner-b-${STAMP}@himate.test"
V_EMAIL="modules-viewer-${STAMP}@himate.test"
A_PASSWORD="PermA5!${STAMP}Access"
B_PASSWORD="PermB5!${STAMP}Access"
V_PASSWORD="PermV5!${STAMP}Access"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

admin_login="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$admin_login" "$BASE_URL/api/v1/auth/login" >/dev/null

printf '23.11.5 synchronized release... '
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
 "display_name":"START 23.11.5 Permissions "+suffix+" "+stamp,
 "legal_name":"START 23.11.5 Permissions "+suffix+" LLC",
 "brand_name":"Permissions "+suffix,
 "category_id":"cat_006","lifecycle":"PROSPECT",
 "contact_name":"Permissions Owner "+suffix,"contact_email":email,
 "country":"United States","state_region":"New York","city":"New York",
 "onboarding_request_id":"start-23-11-5-"+suffix.lower()+"-"+stamp
}))
PY
)"
  curl -fsS -b "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/partners"
}

printf 'create two isolated Golden Test partner tenants... '
A="$(create_partner "$A_EMAIL" A)"
B="$(create_partner "$B_EMAIL" B)"
A_ID="$(printf '%s' "$A" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
B_ID="$(printf '%s' "$B" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
test "$A_ID" != "$B_ID"
curl -fsS -b "$ADMIN_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"START-23.11.5 entitlement intersection"}' "$BASE_URL/api/v1/partners/$A_ID" >/dev/null
curl -fsS -b "$ADMIN_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"START-23.11.5 tenant isolation"}' "$BASE_URL/api/v1/partners/$B_ID" >/dev/null
echo ok

create_owner() {
  partner="$1"; email="$2"; password="$3"; name="$4"
  payload="$(python3 - "$email" "$password" "$name" <<'PY'
import json,sys
print(json.dumps({"name":sys.argv[3],"email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
  curl -fsS -b "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/partners/$partner/portal-users"
}
OWNER_A="$(create_owner "$A_ID" "$A_EMAIL" "$A_PASSWORD" "Permissions Owner A")"
OWNER_B="$(create_owner "$B_ID" "$B_EMAIL" "$B_PASSWORD" "Permissions Owner B")"
OWNER_A_ID="$(printf '%s' "$OWNER_A" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
OWNER_B_ID="$(printf '%s' "$OWNER_B" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"

partner_login() {
  cookie="$1"; email="$2"; password="$3"
  payload="$(python3 - "$email" "$password" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
}
partner_login "$OWNER_A_COOKIE" "$A_EMAIL" "$A_PASSWORD"
partner_login "$OWNER_B_COOKIE" "$B_EMAIL" "$B_PASSWORD"

printf 'owner creates an own-tenant viewer with ALL_OWNED default... '
VIEWER_PAYLOAD="$(python3 - "$V_EMAIL" "$V_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Selected Module Viewer","email":sys.argv[1],"password":sys.argv[2],"role":"viewer"}))
PY
)"
VIEWER="$(curl -fsS -b "$OWNER_A_COOKIE" -H 'Content-Type: application/json' -d "$VIEWER_PAYLOAD" "$BASE_URL/partner/api/v1/users")"
VIEWER_ID="$(printf '%s' "$VIEWER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
partner_login "$VIEWER_COOKIE" "$V_EMAIL" "$V_PASSWORD"
DEFAULT_POLICY="$(curl -fsS -b "$OWNER_A_COOKIE" "$BASE_URL/partner/api/v1/users/$VIEWER_ID/modules")"
python3 - "$DEFAULT_POLICY" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["access_mode"]=="ALL_OWNED",d
assert d["owned_count"]>=1,d
assert d["effective_count"]==d["owned_count"],d
assert "finance" in d["effective_module_keys"],d
PY
echo ok

printf 'ALL_OWNED viewer receives all owned modules while the dynamically-sized catalog remains discoverable... '
VIEWER_MODULES="$(curl -fsS -b "$VIEWER_COOKIE" "$BASE_URL/partner/api/v1/modules")"
python3 - "$VIEWER_MODULES" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); items=d["items"]
assert len(items)>=1,items; assert len({x["key"] for x in items})==len(items),items
finance=next(x for x in items if x["key"]=="finance")
workflow=next(x for x in items if x["key"]=="workshop_workflow")
planned=[x for x in items if x["key"] in {"needs_assessment","two_factor_authentication"}]
assert finance["access_state"]=="ACTIVE" and finance["user_executable"] is True,finance
assert workflow["access_state"]=="ACTIVE" and workflow["user_executable"] is True,workflow
assert {"needs_assessment","two_factor_authentication"}.issubset({x["key"] for x in planned}),planned; assert all(x["access_state"]=="COMING_SOON" and x["user_executable"] is False for x in planned),planned
assert d["user_module_access"]["access_mode"]=="ALL_OWNED",d
PY
echo ok

printf 'owner narrows viewer to SELECTED finance only... '
SELECTED="$(curl -fsS -b "$OWNER_A_COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"access_mode":"SELECTED","module_keys":["finance"]}' "$BASE_URL/partner/api/v1/users/$VIEWER_ID/modules")"
python3 - "$SELECTED" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["access_mode"]=="SELECTED",d
assert d["selected_module_keys"]==["finance"],d
assert d["effective_module_keys"]==["finance"],d
PY
VIEWER_MODULES="$(curl -fsS -b "$VIEWER_COOKIE" "$BASE_URL/partner/api/v1/modules")"
python3 - "$VIEWER_MODULES" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); items=d["items"]
assert len(items)>=1,items; assert len({x["key"] for x in items})==len(items),items
finance=next(x for x in items if x["key"]=="finance")
workflow=next(x for x in items if x["key"]=="workshop_workflow")
assert finance["user_access_state"]=="GRANTED" and finance["user_executable"] is True,finance
assert workflow["access_state"]=="ACTIVE" and workflow["user_access_state"]=="NOT_ASSIGNED" and workflow["user_executable"] is False,workflow
PY
echo ok

printf 'user list exposes selected-access summary... '
USERS="$(curl -fsS -b "$OWNER_A_COOKIE" "$BASE_URL/partner/api/v1/users")"
python3 - "$USERS" "$VIEWER_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); uid=sys.argv[2]
u=next(x for x in d["items"] if x["id"]==uid)
assert u["module_access_mode"]=="SELECTED",u
assert u["selected_module_count"]==1,u
PY
echo ok

printf 'viewer cannot self-grant module access... '
CODE="$(status "$VIEWER_COOKIE" PUT "/partner/api/v1/users/$VIEWER_ID/modules" -H 'Content-Type: application/json' -d '{"access_mode":"ALL_OWNED","module_keys":[]}')"
test "$CODE" = "403"
grep -q 'users.write' "$BODY"
echo ok

printf 'partner cannot assign a module it does not own... '
CODE="$(status "$OWNER_A_COOKIE" PUT "/partner/api/v1/users/$VIEWER_ID/modules" -H 'Content-Type: application/json' -d '{"access_mode":"SELECTED","module_keys":["not_real_module"]}')"
test "$CODE" = "409"
grep -q 'MODULE_NOT_OWNED' "$BODY"
echo ok

printf 'cross-tenant Partner Portal user IDs are not addressable... '
CODE="$(status "$OWNER_A_COOKIE" GET "/partner/api/v1/users/$OWNER_B_ID/modules")"
test "$CODE" = "404"
CODE="$(status "$OWNER_A_COOKIE" PUT "/partner/api/v1/users/$OWNER_B_ID/modules" -H 'Content-Type: application/json' -d '{"access_mode":"SELECTED","module_keys":["finance"]}')"
test "$CODE" = "404"
echo ok

printf 'database persists SELECTED mode and explicit grant... '
DB_MODE="$(docker compose exec -T postgres psql -U himate -d himate -At -c "SELECT module_access_mode FROM identity.partner_users WHERE id='$VIEWER_ID' AND partner_id='$A_ID';")"
test "$DB_MODE" = "SELECTED"
DB_GRANT="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' -c "SELECT partner_id,module_key FROM identity.partner_user_modules WHERE user_id='$VIEWER_ID';")"
test "$DB_GRANT" = "$A_ID|finance"
echo ok

printf 'organization module access withdrawal overrides a persisted user grant... '
READY_MODULES="$(curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/modules")"
PROOF_KEY="$(printf '%s' "$READY_MODULES" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[x for x in d["items"] if str(x.get("key","")).startswith("ci.commercial.") and x.get("publication_status")=="PUBLISHED" and x.get("implementation_state")=="READY" and x.get("availability")=="ACTIVE"]; assert xs,xs; print(xs[0]["key"])')"
C_EMAIL="modules-proof-${STAMP}@himate.test"
C_PASSWORD="PermC5!${STAMP}Access"
C="$(create_partner "$C_EMAIL" C)"
C_ID="$(printf '%s' "$C" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$ADMIN_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"status":"ACTIVE","visible":true,"reason":"START-23.11.5 effective-access proof"}' "$BASE_URL/api/v1/partners/$C_ID/modules/$PROOF_KEY" >/dev/null
OWNER_C="$(create_owner "$C_ID" "$C_EMAIL" "$C_PASSWORD" "Permissions Owner C")"
OWNER_C_ID="$(printf '%s' "$OWNER_C" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
partner_login "$OWNER_C_COOKIE" "$C_EMAIL" "$C_PASSWORD"
C_BEFORE="$(curl -fsS -b "$OWNER_C_COOKIE" "$BASE_URL/partner/api/v1/modules")"
python3 - "$C_BEFORE" "$PROOF_KEY" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); key=sys.argv[2]
m=next(x for x in d["items"] if x["key"]==key)
assert m["access_state"]=="ACTIVE" and m["user_executable"] is True,m
PY
SELECTED_C="$(python3 - "$PROOF_KEY" <<'PY'
import json,sys
print(json.dumps({"access_mode":"SELECTED","module_keys":[sys.argv[1]]}))
PY
)"
curl -fsS -b "$OWNER_C_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$SELECTED_C" "$BASE_URL/partner/api/v1/users/$OWNER_C_ID/modules" >/dev/null
curl -fsS -b "$ADMIN_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"status":"MAINTENANCE","reason":"START-23.11.5 organization access withdrawal"}' "$BASE_URL/api/v1/partners/$C_ID/modules/$PROOF_KEY" >/dev/null
C_AFTER="$(curl -fsS -b "$OWNER_C_COOKIE" "$BASE_URL/partner/api/v1/modules")"
python3 - "$C_AFTER" "$PROOF_KEY" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); key=sys.argv[2]
m=next(x for x in d["items"] if x["key"]==key)
assert m["access_state"]!="ACTIVE",m
assert m["user_access_state"]=="ORGANIZATION_LOCKED",m
assert m["user_executable"] is False,m
PY
POLICY_AFTER_LOSS="$(curl -fsS -b "$OWNER_C_COOKIE" "$BASE_URL/partner/api/v1/users/$OWNER_C_ID/modules")"
python3 - "$POLICY_AFTER_LOSS" "$PROOF_KEY" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); key=sys.argv[2]
assert key in d["selected_module_keys"],d
assert key in d["stale_module_keys"],d
assert key not in d["effective_module_keys"],d
PY
echo ok

printf 'module-access mutation is audit persisted... '
sleep 1
AUDIT="$(curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/audit/events?partner_id=$A_ID&limit=100")"
python3 - "$AUDIT" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); actions={x["action"] for x in d["items"]}
assert "PARTNER_USER_MODULE_ACCESS_UPDATED" in actions,actions
PY
echo ok

echo 'HIMATE START-23.11.5 User ↔ Module Permissions smoke passed'
