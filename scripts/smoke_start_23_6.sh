#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_OLD_COOKIE="$TMP_ROOT/himate-236-owner-old.txt"
OWNER_COOKIE="$TMP_ROOT/himate-236-owner.txt"
OWNER_VERIFY_COOKIE="$TMP_ROOT/himate-236-owner-verify.txt"
ADMIN_COOKIE="$TMP_ROOT/himate-236-admin.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-236-partner.txt"
PARTNER_COOKIE_2="$TMP_ROOT/himate-236-partner-2.txt"
BODY="$TMP_ROOT/himate-236-body.json"
rm -f "$OWNER_OLD_COOKIE" "$OWNER_COOKIE" "$OWNER_VERIFY_COOKIE" "$ADMIN_COOKIE" "$PARTNER_COOKIE" "$PARTNER_COOKIE_2" "$BODY"
trap 'rm -f "$OWNER_OLD_COOKIE" "$OWNER_COOKIE" "$OWNER_VERIFY_COOKIE" "$ADMIN_COOKIE" "$PARTNER_COOKIE" "$PARTNER_COOKIE_2" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
RESET_PASSWORD="$(python3 -c 'import secrets; print("Rr6!"+secrets.token_urlsafe(24))')"
ADMIN_PASSWORD="$(python3 -c 'import secrets; print("Aa6!"+secrets.token_urlsafe(24))')"
PARTNER_PASSWORD="$(python3 -c 'import secrets; print("Pp6!"+secrets.token_urlsafe(24))')"
ADMIN_EMAIL="ci-start236-admin-$STAMP@example.com"
PARTNER_EMAIL="ci-start236-partner-$STAMP@example.com"
LEAD_EMAIL="ci-start236-lead-$STAMP@example.com"
ROLE_KEY="ci_236_operator_$STAMP"

admin_login() {
  cookie="$1"; email="$2"; password="$3"
  payload="$(python3 - "$email" "$password" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login"
}

partner_login() {
  cookie="$1"; email="$2"; password="$3"
  payload="$(python3 - "$email" "$password" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/partner/api/v1/auth/login"
}

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-23.6 login and authenticated identity... '
admin_login "$OWNER_OLD_COOKIE" "$OWNER_EMAIL" "$OWNER_PASSWORD" >/dev/null
me="$(curl -fsS -b "$OWNER_OLD_COOKIE" "$BASE_URL/api/v1/auth/me")"
printf '%s' "$me" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["email"].lower()==sys.argv[1].lower(); assert d["system_owner"] is True' "$OWNER_EMAIL"
echo ok

printf 'logout clears administrator session... '
admin_login "$OWNER_VERIFY_COOKIE" "$OWNER_EMAIL" "$OWNER_PASSWORD" >/dev/null
curl -fsS -b "$OWNER_VERIFY_COOKIE" -c "$OWNER_VERIFY_COOKIE" -X POST "$BASE_URL/api/v1/auth/logout" >/dev/null
test "$(status "$OWNER_VERIFY_COOKIE" GET "/api/v1/auth/me")" = "401"
echo ok

printf 'tokenized password reset rotates all existing sessions... '
reset_request="$(python3 - "$OWNER_EMAIL" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1]}))
PY
)"
reset_response="$(curl -fsS -H 'Content-Type: application/json' -d "$reset_request" "$BASE_URL/api/v1/auth/password-reset/request")"
reset_token="$(printf '%s' "$reset_response" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["accepted"] is True; print(d["development_token"])')"
test -n "$reset_token"
confirm_payload="$(python3 - "$reset_token" "$RESET_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"token":sys.argv[1],"new_password":sys.argv[2]}))
PY
)"
curl -fsS -H 'Content-Type: application/json' -d "$confirm_payload" "$BASE_URL/api/v1/auth/password-reset/confirm" >/dev/null
test "$(status "$OWNER_OLD_COOKIE" GET "/api/v1/auth/me")" = "401"
replay_code="$(curl -sS -o "$BODY" -w '%{http_code}' -H 'Content-Type: application/json' -d "$confirm_payload" "$BASE_URL/api/v1/auth/password-reset/confirm")"
test "$replay_code" = "400"
grep -q 'INVALID_RESET_TOKEN' "$BODY"
admin_login "$OWNER_COOKIE" "$OWNER_EMAIL" "$RESET_PASSWORD" >/dev/null
echo ok

printf 'profile password path restores bootstrap credential and rotates session version... '
restore_payload="$(python3 - "$RESET_PASSWORD" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"current_password":sys.argv[1],"new_password":sys.argv[2]}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$restore_payload" "$BASE_URL/api/v1/profile/password" >/dev/null
curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/auth/me" >/dev/null
admin_login "$OWNER_VERIFY_COOKIE" "$OWNER_EMAIL" "$OWNER_PASSWORD" >/dev/null
echo ok

printf 'custom role create, assignment and backend authorization... '
role_payload="$(python3 - "$ROLE_KEY" <<'PY'
import json,sys
print(json.dumps({
 "key":sys.argv[1],
 "label_en":"START 23.6 Operator",
 "label_hu":"START 23.6 Operátor",
 "description_en":"Administration identity acceptance role",
 "description_hu":"Adminisztrációs identitás elfogadási szerepkör",
 "permissions":["partners.read","notifications.read"]
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$role_payload" "$BASE_URL/api/v1/admin/roles" >/dev/null
admin_payload="$(python3 - "$ADMIN_EMAIL" "$ADMIN_PASSWORD" "$ROLE_KEY" <<'PY'
import json,sys
print(json.dumps({"name":"START 23.6 Administrator","email":sys.argv[1],"password":sys.argv[2],"roles":[sys.argv[3]]}))
PY
)"
admin_created="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$admin_payload" "$BASE_URL/api/v1/admin/users")"
admin_id="$(printf '%s' "$admin_created" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["active"] is True; assert d["roles"]==[sys.argv[1]]; print(d["id"])' "$ROLE_KEY")"
admin_login "$ADMIN_COOKIE" "$ADMIN_EMAIL" "$ADMIN_PASSWORD" >/dev/null
curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/partners?limit=1" >/dev/null
test "$(status "$ADMIN_COOKIE" GET "/api/v1/billing/profile")" = "403"
echo ok

printf 'custom role edit changes authorization without re-login... '
role_patch="$(python3 - <<'PY'
import json
print(json.dumps({"permissions":["partners.read","notifications.read","billing.read"]}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$role_patch" "$BASE_URL/api/v1/admin/roles/$ROLE_KEY" >/dev/null
curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/billing/profile" >/dev/null
echo ok

printf 'administrator suspension invalidates active session and blocks sign-in... '
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"active":false}' "$BASE_URL/api/v1/admin/users/$admin_id" >/dev/null
test "$(status "$ADMIN_COOKIE" GET "/api/v1/auth/me")" = "401"
admin_denied="$(python3 - "$ADMIN_EMAIL" "$ADMIN_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
test "$(curl -sS -o "$BODY" -w '%{http_code}' -H 'Content-Type: application/json' -d "$admin_denied" "$BASE_URL/api/v1/auth/login")" = "401"
echo ok

printf 'partner create, profile mutation and lifecycle state machine... '
partner_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({
 "display_name":"START 23.6 Partner "+s,
 "legal_name":"START 23.6 Partner LLC "+s,
 "brand_name":"START236 "+s,
 "contact_name":"Portal Owner",
 "contact_email":"owner-"+s+"@example.com",
 "country":"US"
}))
PY
)"
partner_created="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner_created" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle"]=="PROSPECT"; print(d["id"])')"
partner_patch="$(python3 - <<'PY'
import json
print(json.dumps({
 "lifecycle":"LICENSE_PENDING",
 "city":"New York",
 "phone":"+1 212 555 2360",
 "finance_contact_name":"Finance Acceptance",
 "finance_contact_email":"finance-236@example.com",
 "reason":"START-23.6 lifecycle acceptance"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$partner_patch" "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
partner_read="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_id")"
printf '%s' "$partner_read" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle"]=="LICENSE_PENDING"; assert d["city"]=="New York"; assert d["finance_contact_name"]=="Finance Acceptance"'
echo ok

printf 'Partner Portal user creation and tenant login... '
portal_payload="$(python3 - "$PARTNER_EMAIL" "$PARTNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 23.6 Portal Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
portal_created="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_payload" "$BASE_URL/api/v1/partners/$partner_id/portal-users")"
portal_id="$(printf '%s' "$portal_created" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["role"]=="owner"; print(d["id"])')"
partner_login "$PARTNER_COOKIE" "$PARTNER_EMAIL" "$PARTNER_PASSWORD" >/dev/null
portal_me="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/auth/me")"
printf '%s' "$portal_me" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]; assert d["role"]=="owner"' "$partner_id"
echo ok

printf 'Partner Portal role/status changes invalidate prior sessions... '
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"role":"viewer"}' "$BASE_URL/api/v1/partners/$partner_id/portal-users/$portal_id" >/dev/null
test "$(status "$PARTNER_COOKIE" GET "/partner/api/v1/auth/me")" = "401"
partner_login "$PARTNER_COOKIE_2" "$PARTNER_EMAIL" "$PARTNER_PASSWORD" >/dev/null
viewer_me="$(curl -fsS -b "$PARTNER_COOKIE_2" "$BASE_URL/partner/api/v1/auth/me")"
printf '%s' "$viewer_me" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["role"]=="viewer"'
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"active":false}' "$BASE_URL/api/v1/partners/$partner_id/portal-users/$portal_id" >/dev/null
test "$(status "$PARTNER_COOKIE_2" GET "/partner/api/v1/auth/me")" = "401"
portal_login_payload="$(python3 - "$PARTNER_EMAIL" "$PARTNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
test "$(curl -sS -o "$BODY" -w '%{http_code}' -H 'Content-Type: application/json' -d "$portal_login_payload" "$BASE_URL/partner/api/v1/auth/login")" = "401"
echo ok

printf 'authoritative HIMATE company profile mutates, reloads and restores... '
company_before="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/profile")"
company_changed="$(printf '%s' "$company_before" | python3 -c 'import json,sys; d=json.load(sys.stdin); d["contact_name"]="START 23.6 Acceptance "+sys.argv[1]; print(json.dumps(d))' "$STAMP")"
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$company_changed" "$BASE_URL/api/v1/billing/profile" >/dev/null
company_after="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/profile")"
printf '%s' "$company_after" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["contact_name"]=="START 23.6 Acceptance "+sys.argv[1]' "$STAMP"
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$company_before" "$BASE_URL/api/v1/billing/profile" >/dev/null
company_restored="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/profile")"
printf '%s' "$company_restored" | python3 -c 'import json,sys; before=json.loads(sys.argv[1]); after=json.load(sys.stdin); assert after["contact_name"]==before["contact_name"]' "$company_before"
echo ok

printf 'signed-in profile mutation persists and restores... '
profile_before="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/profile")"
profile_patch="$(python3 - "$STAMP" <<'PY'
import json,sys
print(json.dumps({"job_title":"START 23.6 Owner "+sys.argv[1],"phone":"+1 212 555 2366"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$profile_patch" "$BASE_URL/api/v1/profile" >/dev/null
profile_after="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/profile")"
printf '%s' "$profile_after" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["job_title"]=="START 23.6 Owner "+sys.argv[1]; assert d["phone"]=="+1 212 555 2366"' "$STAMP"
profile_restore="$(printf '%s' "$profile_before" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(json.dumps({"name":d.get("name",""),"email":d.get("email",""),"preferred_locale":d.get("preferred_locale","en_US"),"timezone":d.get("timezone","UTC"),"job_title":d.get("job_title",""),"phone":d.get("phone","")}))')"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$profile_restore" "$BASE_URL/api/v1/profile" >/dev/null
echo ok

printf 'public contact persists and administrator lead workflow updates... '
lead_payload="$(python3 - "$LEAD_EMAIL" <<'PY'
import json,sys
print(json.dumps({
 "name":"START 23.6 Contact Lead",
 "organization":"HIMATE CI",
 "email":sys.argv[1],
 "message":"START 23.6 verifies the persisted contact lead administration workflow.",
 "website":""
}))
PY
)"
lead_created="$(curl -fsS -H 'Content-Type: application/json' -d "$lead_payload" "$BASE_URL/api/v1/public/contact")"
lead_id="$(printf '%s' "$lead_created" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["received"] is True; print(d["id"])')"
lead_updated="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"lead_status":"CONTACTED","assigned_to":"START 23.6 Owner","admin_note":"Identity and business CRUD acceptance."}' "$BASE_URL/api/v1/contact/inquiries/$lead_id")"
printf '%s' "$lead_updated" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lead_status"]=="CONTACTED"; assert d["assigned_to"]=="START 23.6 Owner"'
echo ok

printf 'notification single-read and read-all persistence... '
sleep 2
notifications="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/notifications?limit=100")"
notification_id="$(printf '%s' "$notifications" | python3 -c 'import json,sys; d=json.load(sys.stdin); unread=[x for x in d["items"] if not x["read"]]; assert unread, d; print(unread[0]["id"])')"
curl -fsS -b "$OWNER_COOKIE" -X POST "$BASE_URL/api/v1/notifications/$notification_id/read" >/dev/null
after_one="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/notifications?limit=100")"
printf '%s' "$after_one" | python3 -c 'import json,sys; d=json.load(sys.stdin); nid=int(sys.argv[1]); x=next(x for x in d["items"] if int(x["id"])==nid); assert x["read"] is True' "$notification_id"
curl -fsS -b "$OWNER_COOKIE" -X POST "$BASE_URL/api/v1/notifications/read-all" >/dev/null
after_all="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/notifications?limit=100")"
printf '%s' "$after_all" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["unread_count"]==0,d'
echo ok

printf 'representative START-23.6 mutations reached central audit... '
sleep 2
audit="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/audit/events?limit=200")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); actions={x["action"] for x in d["items"]}; required={"PASSWORD_RESET_COMPLETED","PROFILE_PASSWORD_CHANGED","ADMIN_ROLE_CREATED","ADMIN_ROLE_UPDATED","ADMIN_USER_CREATED","ADMIN_USER_UPDATED","PROFILE_UPDATED","CONTACT_LEAD_UPDATED"}; missing=required-actions; assert not missing,(missing,actions)'
echo ok

echo "HIMATE START-23.6 Administration, Identity & Business CRUD smoke passed"
