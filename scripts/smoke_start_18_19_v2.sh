#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
PLATFORM_COOKIE="$TMP_ROOT/himate-start-18-19-platform-v2.txt"
OPS_COOKIE="$TMP_ROOT/himate-start-18-19-ops-v2.txt"
FIN_COOKIE="$TMP_ROOT/himate-start-18-19-fin-v2.txt"
REPORT_COOKIE="$TMP_ROOT/himate-start-18-19-report-v2.txt"
BODY="$TMP_ROOT/himate-start-18-19-body-v2.json"
LOGO="$TMP_ROOT/himate-start-18-19-wordmark-2026.webp"
LOGO_HEADERS="$TMP_ROOT/himate-start-18-19-wordmark-2026.headers"
rm -f "$PLATFORM_COOKIE" "$OPS_COOKIE" "$FIN_COOKIE" "$REPORT_COOKIE" "$BODY" "$LOGO" "$LOGO_HEADERS"
trap 'rm -f "$PLATFORM_COOKIE" "$OPS_COOKIE" "$FIN_COOKIE" "$REPORT_COOKIE" "$BODY" "$LOGO" "$LOGO_HEADERS"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
BOOTSTRAP_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
BOOTSTRAP_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
test -n "$BOOTSTRAP_EMAIL"
test "${#BOOTSTRAP_PASSWORD}" -ge 12
OPS_PASSWORD="$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"
FIN_PASSWORD="$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"
REPORT_PASSWORD="$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

json_login_payload() {
  python3 - "$1" "$2" "$3" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":sys.argv[3].lower()=="true"}))
PY
}

login() {
  cookie="$1"
  email="$2"
  password="$3"
  remember="${4:-false}"
  payload="$(json_login_payload "$email" "$password" "$remember")"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login"
}

status() {
  cookie="$1"
  method="$2"
  path="$3"
  shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'bootstrap smoke credentials resolved... '
test -n "$BOOTSTRAP_EMAIL"
test "${#BOOTSTRAP_PASSWORD}" -ge 12
echo ok

printf 'current HIMATE wordmark asset serves a real WebP... '
curl -fsS -D "$LOGO_HEADERS" -o "$LOGO" "$BASE_URL/brand/himate_identity_wordmark_2026.webp"
grep -qi '^Content-Type: image/webp' "$LOGO_HEADERS"
python3 - "$LOGO" <<'PY'
import pathlib,sys
data=pathlib.Path(sys.argv[1]).read_bytes()
assert data[:4] == b"RIFF", data[:16]
assert data[8:12] == b"WEBP", data[:16]
assert len(data) > 100
PY
echo ok

printf 'persistent Remember me session... '
platform_user="$(login "$PLATFORM_COOKIE" "$BOOTSTRAP_EMAIL" "$BOOTSTRAP_PASSWORD" true)"
printf '%s' "$platform_user" | grep -q '"platform_admin"'
python3 - "$PLATFORM_COOKIE" <<'PY'
import pathlib,sys,time
session=[]
for raw in pathlib.Path(sys.argv[1]).read_text().splitlines():
    if not raw:
        continue
    if raw.startswith("#HttpOnly_"):
        raw=raw[len("#HttpOnly_"):]
    elif raw.startswith("#"):
        continue
    parts=raw.split("\t")
    if len(parts) >= 7 and parts[5] == "himate_session":
        session.append(parts)
assert session, "persistent HttpOnly session cookie missing"
expiry=int(session[-1][4])
assert expiry > time.time() + 25*24*3600, f"remember expiry too short: {expiry}"
PY
echo ok

printf 'role catalog exposes START-19 matrix... '
roles="$(curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/admin/roles")"
printf '%s' "$roles" | python3 -c 'import json,sys; d=json.load(sys.stdin); m={x["key"]:x for x in d["items"]}; assert set(m)=={"platform_admin","operations_admin","finance_admin","reporting_admin"}; assert "*" in m["platform_admin"]["permissions"]; assert "billing.approve" in m["finance_admin"]["permissions"]; assert "provisioning.approve" in m["operations_admin"]["permissions"]; assert "evidence.approve" in m["reporting_admin"]["permissions"]; assert "billing.write" not in m["operations_admin"]["permissions"]'
echo ok

printf 'create scoped administrators... '
ops_payload="$(python3 - "$OPS_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"CI Operations Admin","email":"ci-operations-v2@example.com","password":sys.argv[1],"roles":["operations_admin"]}))
PY
)"
fin_payload="$(python3 - "$FIN_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"CI Finance Admin","email":"ci-finance-v2@example.com","password":sys.argv[1],"roles":["finance_admin"]}))
PY
)"
report_payload="$(python3 - "$REPORT_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"CI Reporting Admin","email":"ci-reporting-v2@example.com","password":sys.argv[1],"roles":["reporting_admin"]}))
PY
)"
ops="$(curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json' -d "$ops_payload" "$BASE_URL/api/v1/admin/users")"
ops_id="$(printf '%s' "$ops" | json_field id)"
fin="$(curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json' -d "$fin_payload" "$BASE_URL/api/v1/admin/users")"
fin_id="$(printf '%s' "$fin" | json_field id)"
report="$(curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json' -d "$report_payload" "$BASE_URL/api/v1/admin/users")"
report_id="$(printf '%s' "$report" | json_field id)"
test -n "$ops_id"
test -n "$fin_id"
test -n "$report_id"
echo ok

printf 'last active Platform Admin is protected... '
bootstrap_id="$(printf '%s' "$platform_user" | json_field id)"
last_admin_code="$(status "$PLATFORM_COOKIE" PATCH "/api/v1/admin/users/$bootstrap_id" -H 'Content-Type: application/json' -d '{"active":false}')"
test "$last_admin_code" = "409"
grep -q 'LAST_PLATFORM_ADMIN' "$BODY"
echo ok

printf 'scoped admin logins expose effective permissions... '
ops_login="$(login "$OPS_COOKIE" "ci-operations-v2@example.com" "$OPS_PASSWORD")"
printf '%s' "$ops_login" | grep -q '"provisioning.approve"'
fin_login="$(login "$FIN_COOKIE" "ci-finance-v2@example.com" "$FIN_PASSWORD")"
printf '%s' "$fin_login" | grep -q '"billing.approve"'
report_login="$(login "$REPORT_COOKIE" "ci-reporting-v2@example.com" "$REPORT_PASSWORD")"
printf '%s' "$report_login" | grep -q '"evidence.approve"'
echo ok

printf 'create RBAC test partner... '
partner="$(curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json' \
  -d '{"display_name":"START 19 RBAC Partner V2","legal_name":"START 19 RBAC Partner V2 LLC","category_id":"cat_006","lifecycle":"PROSPECT","contact_name":"RBAC CI","contact_email":"rbac-ci-v2@example.com","country":"United States"}' \
  "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | json_field id)"
test -n "$partner_id"
echo ok

printf 'Operations Admin boundaries... '
curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/system-health/snapshot" >/dev/null
curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/partners?limit=1" >/dev/null
test "$(status "$OPS_COOKIE" GET "/api/v1/billing/profile")" = "403"
test "$(status "$OPS_COOKIE" GET "/api/v1/admin/users")" = "403"
test "$(status "$OPS_COOKIE" POST "/api/v1/provisioning/jobs/ci-missing/run")" != "403"
echo ok

printf 'Finance Admin boundaries and write access... '
curl -fsS -b "$FIN_COOKIE" "$BASE_URL/api/v1/billing/profile" >/dev/null
curl -fsS -b "$FIN_COOKIE" "$BASE_URL/api/v1/modules" >/dev/null
finance_terms="$(status "$FIN_COOKIE" PUT "/api/v1/billing/partners/$partner_id/terms" \
  -H 'Content-Type: application/json' \
  -d '{"currency":"USD","activation_fee":13000,"activation_fee_waived":false,"activation_fee_reason":"","base_monthly_fee":250,"annual_increase_percent":10,"price_effective_from":"2026-09-20","service_anchor_date":"2026-09-20","reason":"START-19 RBAC CI V2"}')"
test "$finance_terms" = "200"
test "$(status "$FIN_COOKIE" GET "/api/v1/system-health/snapshot")" = "403"
test "$(status "$FIN_COOKIE" GET "/api/v1/admin/users")" = "403"
echo ok

printf 'Reporting Admin boundaries and write access... '
curl -fsS -b "$REPORT_COOKIE" "$BASE_URL/api/v1/impact/definitions" >/dev/null
metric_code="$(status "$REPORT_COOKIE" POST "/api/v1/impact/definitions" \
  -H 'Content-Type: application/json' \
  -d '{"metric_key":"ci.rbac.v2","label":"RBAC CI Metric V2","description":"START-19 reporting write boundary","unit":"count","aggregation":"SUM","scope":"PARTNER"}')"
test "$metric_code" = "201" || test "$metric_code" = "409"
test "$(status "$REPORT_COOKIE" GET "/api/v1/billing/profile")" = "403"
test "$(status "$REPORT_COOKIE" GET "/api/v1/cms/pages")" = "403"
test "$(status "$REPORT_COOKIE" PATCH "/api/v1/evidence/evd-missing" -H 'Content-Type: application/json' -d '{"verification_status":"VERIFIED"}')" != "403"
echo ok

printf 'suspended administrator loses access immediately... '
test "$(status "$PLATFORM_COOKIE" PATCH "/api/v1/admin/users/$report_id" -H 'Content-Type: application/json' -d '{"active":false}')" = "200"
test "$(status "$REPORT_COOKIE" GET "/api/v1/impact/definitions")" = "401"
curl -fsS -b "$PLATFORM_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"active":true}' "$BASE_URL/api/v1/admin/users/$report_id" >/dev/null
echo ok

printf 'central audit records semantic, correlated old/new state... '
sleep 1
audit="$(curl -fsS -b "$PLATFORM_COOKIE" --get   --data-urlencode 'q=admin/users'   --data-urlencode 'from=2000-01-01T00:00:00Z'   --data-urlencode 'limit=100'   "$BASE_URL/api/v1/audit/events")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d["items"]; assert d["total"]>=4,d; assert all(x["actor_id"] for x in items); assert all(x["correlation_id"] for x in items); assert any(x["action"]=="ADMIN_USER_CREATED" and x["method"]=="POST" and x["outcome"]=="SUCCESS" for x in items); assert any(x["action"]=="ADMIN_USER_UPDATED" and x["status"]==409 and x["outcome"]=="FAILED" for x in items); changed=[x for x in items if x["action"]=="ADMIN_USER_UPDATED" and isinstance(x.get("old_state"),dict) and isinstance(x.get("new_state"),dict) and x["old_state"].get("active") is True and x["new_state"].get("active") is False]; assert changed,items'
if printf '%s' "$audit" | grep -qi 'local-development-password'; then exit 1; fi
echo ok

printf 'audit action and correlation filters work... '
corr="$(printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(next(x["correlation_id"] for x in d["items"] if x["action"]=="ADMIN_USER_CREATED"))')"
filtered="$(curl -fsS -b "$PLATFORM_COOKIE" --get   --data-urlencode 'action=ADMIN_USER_CREATED'   --data-urlencode "correlation_id=$corr"   "$BASE_URL/api/v1/audit/events")"
printf '%s' "$filtered" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["count"]>=1; assert all(x["action"]=="ADMIN_USER_CREATED" for x in d["items"]); assert all(x["correlation_id"] for x in d["items"])'
echo ok

printf 'central audit table is append-only at database level... '
if docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1   -c "UPDATE identity.audit_events SET outcome='TAMPERED' WHERE id=(SELECT max(id) FROM identity.audit_events);" >/dev/null 2>&1; then
  echo "audit UPDATE unexpectedly succeeded"
  exit 1
fi
echo ok

printf 'administration user list remains consistent... '
users="$(curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/admin/users")"
printf '%s' "$users" | python3 -c 'import json,sys; d=json.load(sys.stdin); emails={x["email"]:x for x in d["items"]}; assert emails["ci-operations-v2@example.com"]["roles"]==["operations_admin"]; assert emails["ci-finance-v2@example.com"]["active"] is True; assert emails["ci-reporting-v2@example.com"]["active"] is True'
echo ok

echo "HIMATE START-18-19 governance, immutable audit and RBAC smoke V3 passed"
