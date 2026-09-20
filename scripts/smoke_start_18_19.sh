#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
PLATFORM_COOKIE="${TMPDIR:-/tmp}/himate-start-18-19-platform.txt"
OPS_COOKIE="${TMPDIR:-/tmp}/himate-start-18-19-ops.txt"
FIN_COOKIE="${TMPDIR:-/tmp}/himate-start-18-19-fin.txt"
REPORT_COOKIE="${TMPDIR:-/tmp}/himate-start-18-19-report.txt"
BODY="${TMPDIR:-/tmp}/himate-start-18-19-body.json"
LOGO="${TMPDIR:-/tmp}/himate-start-18-19-logo.webp"
LOGO_HEADERS="${TMPDIR:-/tmp}/himate-start-18-19-logo.headers"
rm -f "$PLATFORM_COOKIE" "$OPS_COOKIE" "$FIN_COOKIE" "$REPORT_COOKIE" "$BODY" "$LOGO" "$LOGO_HEADERS"
trap 'rm -f "$PLATFORM_COOKIE" "$OPS_COOKIE" "$FIN_COOKIE" "$REPORT_COOKIE" "$BODY" "$LOGO" "$LOGO_HEADERS"' EXIT

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

status() {
  cookie="$1"
  method="$2"
  path="$3"
  shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

login() {
  cookie="$1"
  email="$2"
  password="$3"
  remember="${4:-false}"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"$password\",\"remember\":$remember}" \
    "$BASE_URL/api/v1/auth/login"
}

printf 'brand logo endpoint serves a real WebP... '
curl -fsS -D "$LOGO_HEADERS" -o "$LOGO" "$BASE_URL/art/himate_logo_master_v2.webp"
grep -qi '^Content-Type: image/webp' "$LOGO_HEADERS"
python3 - "$LOGO" <<'PY'
import pathlib,sys
data=pathlib.Path(sys.argv[1]).read_bytes()
assert data[:4] == b"RIFF", data[:16]
assert data[8:12] == b"WEBP", data[:16]
assert len(data) > 100
PY
echo ok

printf 'platform login with persistent Remember me... '
platform_user="$(login "$PLATFORM_COOKIE" "admin@example.com" "local-development-password" true)"
printf '%s' "$platform_user" | grep -q '"platform_admin"'
python3 - "$PLATFORM_COOKIE" <<'PY'
import pathlib,sys,time
lines=[line for line in pathlib.Path(sys.argv[1]).read_text().splitlines() if line and not line.startswith("#")]
session=[line.split("\t") for line in lines if "\thimate_session\t" in line]
assert session, "persistent session cookie missing"
expiry=int(session[-1][4])
assert expiry > time.time() + 25*24*3600, f"remember expiry too short: {expiry}"
PY
echo ok

printf 'role catalog exposes START-19 matrix... '
roles="$(curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/admin/roles")"
printf '%s' "$roles" | python3 -c 'import json,sys; d=json.load(sys.stdin); m={x["key"]:x for x in d["items"]}; assert set(m)=={"platform_admin","operations_admin","finance_admin","reporting_admin"}; assert "*" in m["platform_admin"]["permissions"]; assert "billing.approve" in m["finance_admin"]["permissions"]; assert "provisioning.approve" in m["operations_admin"]["permissions"]; assert "evidence.approve" in m["reporting_admin"]["permissions"]; assert "billing.write" not in m["operations_admin"]["permissions"]'
echo ok

printf 'create scoped administrators... '
ops="$(curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json'   -d '{"name":"CI Operations Admin","email":"ci-operations@example.com","password":"operations-development-password","roles":["operations_admin"]}'   "$BASE_URL/api/v1/admin/users")"
ops_id="$(printf '%s' "$ops" | json_field id)"
fin="$(curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json'   -d '{"name":"CI Finance Admin","email":"ci-finance@example.com","password":"finance-development-password","roles":["finance_admin"]}'   "$BASE_URL/api/v1/admin/users")"
fin_id="$(printf '%s' "$fin" | json_field id)"
report="$(curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json'   -d '{"name":"CI Reporting Admin","email":"ci-reporting@example.com","password":"reporting-development-password","roles":["reporting_admin"]}'   "$BASE_URL/api/v1/admin/users")"
report_id="$(printf '%s' "$report" | json_field id)"
test -n "$ops_id"
test -n "$fin_id"
test -n "$report_id"
echo ok

printf 'last active Platform Admin is protected... '
last_admin_code="$(status "$PLATFORM_COOKIE" PATCH "/api/v1/admin/users/usr_bootstrap_001" -H 'Content-Type: application/json' -d '{"active":false}')"
test "$last_admin_code" = "409"
grep -q 'LAST_PLATFORM_ADMIN' "$BODY"
echo ok

printf 'scoped admin logins return effective permissions... '
ops_login="$(login "$OPS_COOKIE" "ci-operations@example.com" "operations-development-password")"
printf '%s' "$ops_login" | grep -q '"provisioning.approve"'
fin_login="$(login "$FIN_COOKIE" "ci-finance@example.com" "finance-development-password")"
printf '%s' "$fin_login" | grep -q '"billing.approve"'
report_login="$(login "$REPORT_COOKIE" "ci-reporting@example.com" "reporting-development-password")"
printf '%s' "$report_login" | grep -q '"evidence.approve"'
echo ok

printf 'create RBAC test partner as Platform Admin... '
partner="$(curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json'   -d '{"display_name":"START 19 RBAC Partner","legal_name":"START 19 RBAC Partner LLC","category_id":"cat_006","lifecycle":"PROSPECT","contact_name":"RBAC CI","contact_email":"rbac-ci@example.com","country":"United States"}'   "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | json_field id)"
test -n "$partner_id"
echo ok

printf 'Operations Admin boundaries... '
curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/system-health" >/dev/null
curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/partners?limit=1" >/dev/null
ops_billing="$(status "$OPS_COOKIE" GET "/api/v1/billing/profile")"
test "$ops_billing" = "403"
ops_admin="$(status "$OPS_COOKIE" GET "/api/v1/admin/users")"
test "$ops_admin" = "403"
approve_ops="$(status "$OPS_COOKIE" POST "/api/v1/provisioning/jobs/ci-missing/run")"
test "$approve_ops" != "403"
echo ok

printf 'Finance Admin boundaries and write access... '
curl -fsS -b "$FIN_COOKIE" "$BASE_URL/api/v1/billing/profile" >/dev/null
curl -fsS -b "$FIN_COOKIE" "$BASE_URL/api/v1/modules" >/dev/null
finance_terms="$(status "$FIN_COOKIE" PUT "/api/v1/billing/partners/$partner_id/terms"   -H 'Content-Type: application/json'   -d '{"currency":"USD","activation_fee":13000,"activation_fee_waived":false,"activation_fee_reason":"","base_monthly_fee":250,"annual_increase_percent":10,"price_effective_from":"2026-09-20","service_anchor_date":"2026-09-20","reason":"START-19 RBAC CI"}')"
test "$finance_terms" = "200"
finance_ops="$(status "$FIN_COOKIE" GET "/api/v1/system-health")"
test "$finance_ops" = "403"
finance_admin="$(status "$FIN_COOKIE" GET "/api/v1/admin/users")"
test "$finance_admin" = "403"
echo ok

printf 'Reporting Admin boundaries and write access... '
curl -fsS -b "$REPORT_COOKIE" "$BASE_URL/api/v1/impact/definitions" >/dev/null
metric_code="$(status "$REPORT_COOKIE" POST "/api/v1/impact/definitions"   -H 'Content-Type: application/json'   -d '{"metric_key":"ci.rbac","label":"RBAC CI Metric","description":"START-19 reporting write boundary","unit":"count","aggregation":"SUM","scope":"PARTNER"}')"
test "$metric_code" = "201" || test "$metric_code" = "409"
report_billing="$(status "$REPORT_COOKIE" GET "/api/v1/billing/profile")"
test "$report_billing" = "403"
report_cms="$(status "$REPORT_COOKIE" GET "/api/v1/cms/pages")"
test "$report_cms" = "403"
approve_reporting="$(status "$REPORT_COOKIE" PATCH "/api/v1/evidence/evd-missing"   -H 'Content-Type: application/json' -d '{"verification_status":"VERIFIED"}')"
test "$approve_reporting" != "403"
echo ok

printf 'suspended administrator loses access immediately... '
suspend_code="$(status "$PLATFORM_COOKIE" PATCH "/api/v1/admin/users/$report_id"   -H 'Content-Type: application/json' -d '{"active":false}')"
test "$suspend_code" = "200"
suspended_code="$(status "$REPORT_COOKIE" GET "/api/v1/impact/definitions")"
test "$suspended_code" = "401"
curl -fsS -b "$PLATFORM_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"active":true}' "$BASE_URL/api/v1/admin/users/$report_id" >/dev/null
echo ok

printf 'central audit records governance mutations... '
sleep 1
audit="$(curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/audit/events?q=admin%2Fusers&limit=100")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["total"]>=4, d; assert all(x["actor_id"] for x in d["items"]); assert any(x["method"]=="POST" and x["outcome"]=="SUCCESS" for x in d["items"]); assert any(x["status"]==409 and x["outcome"]=="FAILED" for x in d["items"])'
echo ok

printf 'administration user list remains consistent... '
users="$(curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/admin/users")"
printf '%s' "$users" | python3 -c 'import json,sys; d=json.load(sys.stdin); emails={x["email"]:x for x in d["items"]}; assert emails["ci-operations@example.com"]["roles"]==["operations_admin"]; assert emails["ci-finance@example.com"]["active"] is True; assert emails["ci-reporting@example.com"]["active"] is True'
echo ok

echo "HIMATE START-18-19 governance and RBAC smoke passed"
