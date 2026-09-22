#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-222-owner.txt"
PARTNER_A_COOKIE="$TMP_ROOT/himate-222-partner-a.txt"
PARTNER_B_COOKIE="$TMP_ROOT/himate-222-partner-b.txt"
PARTNER_ADMIN_COOKIE="$TMP_ROOT/himate-222-partner-admin.txt"
BODY="$TMP_ROOT/himate-222-body.json"
rm -f "$OWNER_COOKIE" "$PARTNER_A_COOKIE" "$PARTNER_B_COOKIE" "$PARTNER_ADMIN_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_A_COOKIE" "$PARTNER_B_COOKIE" "$PARTNER_ADMIN_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
PARTNER_A_PASSWORD="$(python3 -c 'import secrets; print("Pa1!"+secrets.token_urlsafe(24))')"
PARTNER_B_PASSWORD="$(python3 -c 'import secrets; print("Pb2!"+secrets.token_urlsafe(24))')"
PARTNER_ADMIN_PASSWORD="$(python3 -c 'import secrets; print("Pc3!"+secrets.token_urlsafe(24))')"

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

printf 'START-22.2 system owner login... '
admin_login "$OWNER_COOKIE" "$OWNER_EMAIL" "$OWNER_PASSWORD" >/dev/null
echo ok

printf 'create two isolated Partner Portal tenants... '
partner_a="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"display_name":"START 22.2 Tenant A","legal_name":"START 22.2 Tenant A LLC","brand_name":"Tenant A","contact_name":"Alice Owner","contact_email":"alice-owner@example.com","country":"US"}'   "$BASE_URL/api/v1/partners")"
partner_b="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"display_name":"START 22.2 Tenant B","legal_name":"START 22.2 Tenant B LLC","brand_name":"Tenant B","contact_name":"Bob Owner","contact_email":"bob-owner@example.com","country":"US"}'   "$BASE_URL/api/v1/partners")"
partner_a_id="$(printf '%s' "$partner_a" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
partner_b_id="$(printf '%s' "$partner_b" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
test -n "$partner_a_id"
test -n "$partner_b_id"
test "$partner_a_id" != "$partner_b_id"
echo "$partner_a_id / $partner_b_id"

printf 'register dependency-controlled portal modules before entitlement initialization... '
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"group_key":"ci_222","label":"CI START 22.2","sort_order":92}'   "$BASE_URL/api/v1/module-groups" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"key":"ci.portal_required","label":"Portal Required Module","group_key":"ci_222","description":"Required dependency for portal acceptance","currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":25,"availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY","module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}}'   "$BASE_URL/api/v1/modules" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"key":"ci.portal_feature","label":"Portal Feature Module","group_key":"ci_222","description":"Dependency-gated portal module","currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":40,"availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY","module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}}'   "$BASE_URL/api/v1/modules" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"target_module_key":"ci.portal_required","relation_type":"REQUIRES","note":"START-22.2 acceptance dependency"}'   "$BASE_URL/api/v1/modules/ci.portal_feature/relationships" >/dev/null
for partner_id in "$partner_a_id" "$partner_b_id"; do
  curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'     -d '{"visible":true,"included_in_base":false,"contract_currency":"USD","quote_reference":"CI-START-22.2-REQUIRED","partner_price":25,"partner_activation_fee":0,"reason":"Historical START-22.2 fixture commercial prerequisite"}'     "$BASE_URL/api/v1/partners/$partner_id/modules/ci.portal_required" >/dev/null
  curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'     -d '{"visible":true,"included_in_base":false,"contract_currency":"USD","quote_reference":"CI-START-22.2-FEATURE","partner_price":40,"partner_activation_fee":0,"reason":"Historical START-22.2 fixture commercial prerequisite"}'     "$BASE_URL/api/v1/partners/$partner_id/modules/ci.portal_feature" >/dev/null
done
echo ok

printf 'system owner bootstraps each partner owner account... '
owner_a_payload="$(python3 - "$PARTNER_A_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Alice Portal Owner","email":"ci-start222-owner-a@example.com","password":sys.argv[1],"role":"owner"}))
PY
)"
owner_b_payload="$(python3 - "$PARTNER_B_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Bob Portal Owner","email":"ci-start222-owner-b@example.com","password":sys.argv[1],"role":"owner"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$owner_a_payload" "$BASE_URL/api/v1/partners/$partner_a_id/portal-users" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$owner_b_payload" "$BASE_URL/api/v1/partners/$partner_b_id/portal-users" >/dev/null
echo ok

printf 'identity email cannot cross from Partner Portal into HIMATE administration... '
admin_collision_payload="$(python3 - "$PARTNER_A_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Collision Attempt","email":"ci-start222-owner-a@example.com","password":sys.argv[1],"roles":["operations_admin"]}))
PY
)"
test "$(status "$OWNER_COOKIE" POST "/api/v1/admin/users" -H 'Content-Type: application/json' -d "$admin_collision_payload")" = "409"
grep -q 'EMAIL_EXISTS' "$BODY"
echo ok

printf 'partner sessions carry immutable tenant scope... '
partner_login "$PARTNER_A_COOKIE" "ci-start222-owner-a@example.com" "$PARTNER_A_PASSWORD" >/dev/null
partner_login "$PARTNER_B_COOKIE" "ci-start222-owner-b@example.com" "$PARTNER_B_PASSWORD" >/dev/null
me_a="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/auth/me")"
printf '%s' "$me_a" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]; assert d["role"]=="owner"; assert "*" in d["permissions"]' "$partner_a_id"
test "$(status "$PARTNER_A_COOKIE" GET "/api/v1/partners")" = "401"
echo ok

printf 'tenant A cannot override its session scope to tenant B... '
company_override="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/company?partner_id=$partner_b_id")"
printf '%s' "$company_override" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["id"]==sys.argv[1]; assert d["id"]!=sys.argv[2]' "$partner_a_id" "$partner_b_id"
modules_override="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/modules?partner_id=$partner_b_id")"
printf '%s' "$modules_override" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]' "$partner_a_id"
billing_override="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/billing/summary?partner_id=$partner_b_id")"
printf '%s' "$billing_override" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]' "$partner_a_id"
echo ok

printf 'dependency gate blocks feature until required module is active... '
catalog_a="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$catalog_a" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=next(x for x in d["items"] if x["key"]=="ci.portal_feature"); assert m["can_activate"] is False; assert any("Requires Portal Required Module" in x for x in m["activation_blockers"])'
test "$(status "$PARTNER_A_COOKIE" POST "/partner/api/v1/modules/ci.portal_feature/activate" -H 'Content-Type: application/json' -d '{}')" = "409"
grep -q 'MODULE_DEPENDENCY_BLOCKED' "$BODY"
curl -fsS -b "$PARTNER_A_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/partner/api/v1/modules/ci.portal_required/activate" >/dev/null
feature_activation="$(curl -fsS -b "$PARTNER_A_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/partner/api/v1/modules/ci.portal_feature/activate")"
printf '%s' "$feature_activation" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["module"]["status"]=="ACTIVE"; assert d["module"]["key"]=="ci.portal_feature"'
echo ok

printf 'activation synchronizes a calendar-month subscription and month-boundary cancellation... '
billing_a="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/billing/summary")"
printf '%s' "$billing_a" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]; assert d["current_total"]>=65; assert d["billing_cycle_model"]=="CALENDAR_MONTH"; assert d["cycle_days"] is None; assert d["proration"]=="NONE"' "$partner_a_id"
subs_a="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/billing/subscriptions")"
printf '%s' "$subs_a" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=next(x for x in d["items"] if x["module_key"]=="ci.portal_feature"); assert m["cancel_at_period_end"] is False; assert m["auto_renew"] is True; assert m["billing_model"]=="CALENDAR_MONTH"; assert m["proration"]=="NONE"'
cancelled="$(curl -fsS -b "$PARTNER_A_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"cancel_at_period_end":true}' "$BASE_URL/partner/api/v1/modules/ci.portal_feature/subscription")"
printf '%s' "$cancelled" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["cancel_at_period_end"] is True; assert d["auto_renew"] is False; assert d["period_end_exclusive"]'
modules_after_cancel="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$modules_after_cancel" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=next(x for x in d["items"] if x["key"]=="ci.portal_feature"); assert m["status"]=="ACTIVE"'
echo ok

printf 'partner company update cannot mutate lifecycle authority... '
curl -fsS -b "$PARTNER_A_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"display_name":"START 22.2 Tenant A Updated","city":"New York"}'   "$BASE_URL/partner/api/v1/company" >/dev/null
attempt_status="$(status "$PARTNER_A_COOKIE" PATCH "/partner/api/v1/company" -H 'Content-Type: application/json' -d '{"lifecycle":"LIVE"}')"
test "$attempt_status" = "200" || test "$attempt_status" = "400"
admin_company="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_a_id")"
printf '%s' "$admin_company" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["display_name"]=="START 22.2 Tenant A Updated"; assert d["city"]=="New York"; assert d["lifecycle"]=="PROSPECT"'
echo ok

printf 'partner owner manages users but partner admin cannot assign owner authority... '
admin_payload="$(python3 - "$PARTNER_ADMIN_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Tenant A Portal Admin","email":"ci-start222-admin-a@example.com","password":sys.argv[1],"role":"admin"}))
PY
)"
portal_admin="$(curl -fsS -b "$PARTNER_A_COOKIE" -H 'Content-Type: application/json' -d "$admin_payload" "$BASE_URL/partner/api/v1/users")"
portal_admin_id="$(printf '%s' "$portal_admin" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["role"]=="admin"; print(d["id"])')"
partner_login "$PARTNER_ADMIN_COOKIE" "ci-start222-admin-a@example.com" "$PARTNER_ADMIN_PASSWORD" >/dev/null
owner_attempt="$(python3 - <<'PY'
import json
print(json.dumps({"name":"Escalation Attempt","email":"ci-start222-escalate@example.com","password":"Strong-Pass4!Portal","role":"owner"}))
PY
)"
test "$(status "$PARTNER_ADMIN_COOKIE" POST "/partner/api/v1/users" -H 'Content-Type: application/json' -d "$owner_attempt")" = "403"
grep -q 'Only a partner owner' "$BODY"
test -n "$portal_admin_id"
echo ok

printf 'portal mutations are written to central immutable audit with partner scope... '
sleep 1
audit="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/audit/events?partner_id=$partner_a_id&limit=100")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); actions={x["action"] for x in d["items"]}; assert "PARTNER_MODULE_ACTIVATED" in actions; assert "PARTNER_SUBSCRIPTION_UPDATED" in actions; assert "PARTNER_COMPANY_UPDATED" in actions; assert all(x["partner_id"]==sys.argv[1] for x in d["items"])' "$partner_a_id"
echo ok

printf 'archived partner loses portal API access with an existing session... '
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"lifecycle":"ARCHIVED","reason":"START-22.2 portal access revocation acceptance"}'   "$BASE_URL/api/v1/partners/$partner_a_id" >/dev/null
test "$(status "$PARTNER_A_COOKIE" GET "/partner/api/v1/dashboard")" = "403"
grep -q 'PARTNER_ACCESS_DISABLED' "$BODY"
curl -fsS -b "$PARTNER_B_COOKIE" "$BASE_URL/partner/api/v1/dashboard" >/dev/null
echo ok

echo "HIMATE START-22.2 Partner Portal tenant-isolation and self-service smoke passed"
