#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start231-owner.txt"
USER_COOKIE="$TMP_ROOT/himate-start231-user.txt"
BODY="$TMP_ROOT/himate-start231-body.json"
PROFILE_ORIGINAL="$TMP_ROOT/himate-start231-profile.json"
rm -f "$OWNER_COOKIE" "$USER_COOKIE" "$BODY" "$PROFILE_ORIGINAL"
trap 'rm -f "$OWNER_COOKIE" "$USER_COOKIE" "$BODY" "$PROFILE_ORIGINAL"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
ROLE_KEY="ci_start231_$STAMP"
ADMIN_EMAIL="start231-$STAMP@example.com"
ADMIN_PASSWORD="Start231-$STAMP!Aa"
GROUP_KEY="ci_start231_$STAMP"
MODULE_KEY="ci.start231_$STAMP"
CATEGORY_NAME="START 23.1 Category $STAMP"
LEAD_EMAIL="start231-lead-$STAMP@example.com"

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-23.1 owner login... '
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'partner category mutation persists and reads back... '
category="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d "$(python3 - "$CATEGORY_NAME" <<'PY'
import json,sys
print(json.dumps({"name":sys.argv[1]}))
PY
)" "$BASE_URL/api/v1/partner-categories")"
category_id="$(printf '%s' "$category" | json_field id)"
test -n "$category_id"
categories="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partner-categories")"
printf '%s' "$categories" | python3 - "$category_id" "$CATEGORY_NAME" <<'PY'
import json,sys
d=json.load(sys.stdin)
cid,name=sys.argv[1],sys.argv[2]
assert any(str(x.get("id"))==cid and x.get("name")==name for x in d.get("items",[])), d
PY
echo ok

printf 'module group and module create/update mutations persist... '
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d "$(python3 - "$GROUP_KEY" <<'PY'
import json,sys
print(json.dumps({"group_key":sys.argv[1],"label":"START 23.1 Canary Group","sort_order":231}))
PY
)" "$BASE_URL/api/v1/module-groups" >/dev/null
module_payload="$(python3 - "$MODULE_KEY" "$GROUP_KEY" <<'PY'
import json,sys
print(json.dumps({
  "key":sys.argv[1],
  "label":"START 23.1 Canary Module",
  "group_key":sys.argv[2],
  "description":"Functional contract mutation canary",
  "currency":"USD",
  "version":"1.0.0",
  "latest_version":"1.0.0",
  "default_monthly_price":23.10,
  "availability":"ACTIVE",
  "module_type":"FEATURE",
  "owner_team":"Platform",
  "manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$module_payload" "$BASE_URL/api/v1/modules" >/dev/null
patch_payload='{"label":"START 23.1 Canary Module Updated","description":"Mutation readback verified","default_monthly_price":31.23,"availability":"ACTIVE","latest_version":"1.0.1"}'
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$patch_payload" "$BASE_URL/api/v1/modules/$MODULE_KEY" >/dev/null
modules="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
printf '%s' "$modules" | python3 - "$MODULE_KEY" <<'PY'
import json,sys
d=json.load(sys.stdin); key=sys.argv[1]
m=next(x for x in d["items"] if x["key"]==key)
assert m["label"]=="START 23.1 Canary Module Updated",m
assert abs(float(m["default_monthly_price"])-31.23)<0.001,m
assert m["latest_version"]=="1.0.1",m
PY
echo ok

printf 'custom role mutation persists... '
role_payload="$(python3 - "$ROLE_KEY" <<'PY'
import json,sys
print(json.dumps({
  "key":sys.argv[1],
  "label":"START 23.1 Canary Role",
  "description":"Mutation acceptance canary",
  "permissions":["dashboard.read","partners.read"]
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$role_payload" "$BASE_URL/api/v1/admin/roles" >/dev/null
roles="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/admin/roles")"
printf '%s' "$roles" | python3 - "$ROLE_KEY" <<'PY'
import json,sys
d=json.load(sys.stdin); key=sys.argv[1]
r=next(x for x in d["items"] if x["key"]==key)
assert r["active"] is True,r
assert "dashboard.read" in r["permissions"],r
PY
echo ok

printf 'administrator create, login, authorization and suspension are enforced... '
user_payload="$(python3 - "$ADMIN_EMAIL" "$ADMIN_PASSWORD" "$ROLE_KEY" <<'PY'
import json,sys
print(json.dumps({
  "name":"START 23.1 Canary Admin",
  "email":sys.argv[1],
  "password":sys.argv[2],
  "roles":[sys.argv[3]]
}))
PY
)"
created_user="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$user_payload" "$BASE_URL/api/v1/admin/users")"
user_id="$(printf '%s' "$created_user" | json_field id)"
test -n "$user_id"
users="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/admin/users")"
printf '%s' "$users" | python3 - "$user_id" "$ROLE_KEY" <<'PY'
import json,sys
d=json.load(sys.stdin); uid,key=sys.argv[1],sys.argv[2]
u=next(x for x in d["items"] if x["id"]==uid)
assert key in u["roles"],u
assert u["active"] is True,u
PY

canary_login="$(python3 - "$ADMIN_EMAIL" "$ADMIN_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$USER_COOKIE" -H 'Content-Type: application/json' -d "$canary_login" "$BASE_URL/api/v1/auth/login" >/dev/null
curl -fsS -b "$USER_COOKIE" "$BASE_URL/api/v1/dashboard/summary" >/dev/null
admin_list_code="$(status "$USER_COOKIE" GET "/api/v1/admin/users")"
test "$admin_list_code" = "403"

suspend_payload="$(python3 - "$ADMIN_EMAIL" "$ROLE_KEY" <<'PY'
import json,sys
print(json.dumps({
  "name":"START 23.1 Canary Admin",
  "email":sys.argv[1],
  "roles":[sys.argv[2]],
  "active":False
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$suspend_payload" "$BASE_URL/api/v1/admin/users/$user_id" >/dev/null
revoked_code="$(status "$USER_COOKIE" GET "/api/v1/dashboard/summary")"
case "$revoked_code" in
  401|403) ;;
  *) echo "suspended administrator session still accepted: $revoked_code" >&2; cat "$BODY" >&2; exit 1 ;;
esac
echo ok

printf 'profile mutation persists and is restored... '
curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/profile" > "$PROFILE_ORIGINAL"
update_profile="$(python3 - "$PROFILE_ORIGINAL" <<'PY'
import json,sys
p=json.load(open(sys.argv[1]))
print(json.dumps({
  "name":p.get("name",""),
  "email":p.get("email",""),
  "job_title":"START-23.1 mutation canary",
  "phone":p.get("phone",""),
  "timezone":p.get("timezone","UTC"),
  "preferred_locale":p.get("preferred_locale","en_US")
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$update_profile" "$BASE_URL/api/v1/profile" >/dev/null
changed="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/profile")"
printf '%s' "$changed" | python3 -c 'import json,sys; assert json.load(sys.stdin)["job_title"]=="START-23.1 mutation canary"'
restore_profile="$(python3 - "$PROFILE_ORIGINAL" <<'PY'
import json,sys
p=json.load(open(sys.argv[1]))
print(json.dumps({
  "name":p.get("name",""),
  "email":p.get("email",""),
  "job_title":p.get("job_title",""),
  "phone":p.get("phone",""),
  "timezone":p.get("timezone","UTC"),
  "preferred_locale":p.get("preferred_locale","en_US")
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$restore_profile" "$BASE_URL/api/v1/profile" >/dev/null
echo ok

printf 'public contact -> admin mutation -> audit chain persists... '
lead_payload="$(python3 - "$LEAD_EMAIL" <<'PY'
import json,sys
print(json.dumps({
  "name":"START 23.1 Mutation Lead",
  "organization":"HIMATE CI",
  "email":sys.argv[1],
  "message":"START-23.1 validates a real public write, admin mutation, persistence readback and audit event.",
  "website":""
}))
PY
)"
lead="$(curl -fsS -H 'Content-Type: application/json' -d "$lead_payload" "$BASE_URL/api/v1/public/contact")"
lead_id="$(printf '%s' "$lead" | json_field id)"
test -n "$lead_id"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"lead_status":"CONTACTED","assigned_to":"START-23.1 CI","admin_note":"Mutation canary verified."}'   "$BASE_URL/api/v1/contact/inquiries/$lead_id" >/dev/null
lead_read="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/contact/inquiries/$lead_id")"
printf '%s' "$lead_read" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lead_status"]=="CONTACTED"; assert d["assigned_to"]=="START-23.1 CI"'
sleep 1
audit="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/audit/events?action=CONTACT_LEAD_UPDATED&limit=100")"
printf '%s' "$audit" | python3 - "$lead_id" <<'PY'
import json,sys
d=json.load(sys.stdin); lead_id=sys.argv[1]
assert any(x.get("action")=="CONTACT_LEAD_UPDATED" and x.get("resource")=="contact" for x in d.get("items",[])),d
PY
echo ok

echo "HIMATE START-23.1 mutation canary passed: writes, persistence readback, authorization and audit were exercised."
