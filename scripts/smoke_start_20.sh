#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
PLATFORM_COOKIE="$TMP_ROOT/himate-start20-platform.txt"
OPS_COOKIE="$TMP_ROOT/himate-start20-ops.txt"
FIN_COOKIE="$TMP_ROOT/himate-start20-fin.txt"
BODY="$TMP_ROOT/himate-start20-body.json"
rm -f "$PLATFORM_COOKIE" "$OPS_COOKIE" "$FIN_COOKIE" "$BODY"
trap 'rm -f "$PLATFORM_COOKIE" "$OPS_COOKIE" "$FIN_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
BOOTSTRAP_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
BOOTSTRAP_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
OPS_PASSWORD="$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"
FIN_PASSWORD="$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

login() {
  cookie="$1"
  email="$2"
  password="$3"
  payload="$(python3 - "$email" "$password" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login"
}

status() {
  cookie="$1"
  method="$2"
  path="$3"
  shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-20 bootstrap login... '
login "$PLATFORM_COOKIE" "$BOOTSTRAP_EMAIL" "$BOOTSTRAP_PASSWORD" >/dev/null
echo ok

printf 'create START-20 scoped administrators... '
ops_payload="$(python3 - "$OPS_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 20 Operations","email":"ci-start20-ops@example.com","password":sys.argv[1],"roles":["operations_admin"]}))
PY
)"
fin_payload="$(python3 - "$FIN_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 20 Finance","email":"ci-start20-fin@example.com","password":sys.argv[1],"roles":["finance_admin"]}))
PY
)"
curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json' -d "$ops_payload" "$BASE_URL/api/v1/admin/users" >/dev/null
curl -fsS -b "$PLATFORM_COOKIE" -H 'Content-Type: application/json' -d "$fin_payload" "$BASE_URL/api/v1/admin/users" >/dev/null
login "$OPS_COOKIE" "ci-start20-ops@example.com" "$OPS_PASSWORD" >/dev/null
login "$FIN_COOKIE" "ci-start20-fin@example.com" "$FIN_PASSWORD" >/dev/null
echo ok

printf 'reuse provisioned CONFIGURATION partner... '
configured="$(curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/partners?lifecycle=CONFIGURATION&limit=1&core_only=true&include_stats=false")"
partner_id="$(printf '%s' "$configured" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["items"], d; print(d["items"][0]["id"])')"
test -n "$partner_id"
echo "$partner_id"

printf 'staging environment supports internal-domain state... '
staging_list="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/environments?partner_id=$partner_id")"
staging_id="$(printf '%s' "$staging_list" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(next(x["id"] for x in d["items"] if x["kind"]=="STAGING"))')"
staging_verified="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/environments/$staging_id/verify-domain")"
printf '%s' "$staging_verified" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["domain_status"]=="INTERNAL"; assert d["dns_status"]=="NOT_APPLICABLE"; assert d["tls_status"]=="NOT_APPLICABLE"'
staging_deployed="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d '{"release":"start20-staging"}' "$BASE_URL/api/v1/environments/$staging_id/deploy")"
printf '%s' "$staging_deployed" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["deployment_status"]=="DEPLOYED"; assert d["environment_status"]=="READY"; assert d["active_release"]=="start20-staging"'
echo ok

printf 'production requires explicit public hostname... '
no_host_payload="$(python3 - "$partner_id" <<'PY'
import json,sys
print(json.dumps({"partner_id":sys.argv[1],"kind":"PRODUCTION","desired_release":"start20-prod"}))
PY
)"
test "$(status "$OPS_COOKIE" POST "/api/v1/environments" -H 'Content-Type: application/json' -d "$no_host_payload")" = "400"
echo ok

printf 'create production environment... '
prod_payload="$(python3 - "$partner_id" <<'PY'
import json,sys
print(json.dumps({"partner_id":sys.argv[1],"kind":"PRODUCTION","hostname":"start20-ci.invalid","platform_version":"20.0.0","desired_release":"start20-prod","config":{"source":"ci"}}))
PY
)"
production="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d "$prod_payload" "$BASE_URL/api/v1/environments")"
production_id="$(printf '%s' "$production" | json_field id)"
printf '%s' "$production" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["kind"]=="PRODUCTION"; assert d["domain_status"]=="UNVERIFIED"; assert d["launch_ready"] is False'
echo ok

printf 'Finance cannot access environment controls... '
test "$(status "$FIN_COOKIE" GET "/api/v1/environments")" = "403"
test "$(status "$FIN_COOKIE" POST "/api/v1/environments/$production_id/deploy" -H 'Content-Type: application/json' -d '{}')" = "403"
echo ok

printf 'production deploy advances partner CONFIGURATION to TESTING... '
prod_deployed="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d '{"release":"start20-prod"}' "$BASE_URL/api/v1/environments/$production_id/deploy")"
printf '%s' "$prod_deployed" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["deployment_status"]=="DEPLOYED"; assert d["environment_status"]=="CONFIGURATION_REQUIRED"; assert d["runtime_status"]=="OK"; assert d["active_release"]=="start20-prod"'
curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/partners/$partner_id" | grep -q '"lifecycle":"TESTING"'
test "$(status "$OPS_COOKIE" PATCH "/api/v1/environments/$production_id" -H 'Content-Type: application/json' -d '{"environment_status":"LIVE"}')" = "409"
grep -q 'LAUNCH_GATE' "$BODY"
echo ok

printf 'DNS/TLS verification persists launch blockers... '
verified="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/environments/$production_id/verify-domain")"
printf '%s' "$verified" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["domain_status"]=="FAILED"; assert d["dns_status"]=="FAILED"; assert d["tls_status"]=="SKIPPED"; assert d["launch_ready"] is False; assert d["last_domain_check"]'
test "$(status "$OPS_COOKIE" POST "/api/v1/environments/$production_id/launch" -H 'Content-Type: application/json' -d '{}')" = "409"
grep -q 'LAUNCH_GATE' "$BODY"
echo ok

printf 'simulate externally verified DNS/TLS for positive launch path... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE environments.partner_environments SET dns_status='VERIFIED',tls_status='VERIFIED',domain_status='VERIFIED',domain_error='',last_domain_check=NOW(),environment_status='READY_FOR_LAUNCH',updated_at=NOW() WHERE id='$production_id'" >/dev/null
curl -fsS -b "$PLATFORM_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"lifecycle":"READY_FOR_LAUNCH","reason":"START-20 CI simulated verified external domain"}' "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
ready="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/environments?partner_id=$partner_id")"
printf '%s' "$ready" | python3 -c 'import json,sys; d=json.load(sys.stdin); p=next(x for x in d["items"] if x["kind"]=="PRODUCTION"); assert p["launch_ready"] is True; assert p["environment_status"]=="READY_FOR_LAUNCH"'
echo ok

printf 'READY FOR LAUNCH transitions environment and partner to LIVE... '
live="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/environments/$production_id/launch")"
printf '%s' "$live" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["environment_status"]=="LIVE"; assert d["launch_actor"]; assert d["launched_at"]; assert d["launch_ready"] is True'
curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/partners/$partner_id" | grep -q '"lifecycle":"LIVE"'
echo ok

printf 'environment list exposes START-20 control state... '
envs="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/environments?partner_id=$partner_id")"
printf '%s' "$envs" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["count"]==2; kinds={x["kind"]:x for x in d["items"]}; assert set(kinds)=={"STAGING","PRODUCTION"}; p=kinds["PRODUCTION"]; assert p["environment_status"]=="LIVE"; assert p["domain_status"]=="VERIFIED"; assert "launch_blockers" in p; assert "dns_status" in p; assert "tls_status" in p'
echo ok

printf 'START-20 mutations are present in central audit... '
sleep 1
audit="$(curl -fsS -b "$PLATFORM_COOKIE" "$BASE_URL/api/v1/audit/events?q=environments&limit=100")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["total"]>=8, d; assert any(x["resource"]=="environments" and x["method"]=="POST" and x["outcome"]=="SUCCESS" for x in d["items"]); assert any(x["resource"]=="environments" and x["status"]==409 and x["outcome"]=="FAILED" for x in d["items"])'
echo ok

echo "HIMATE START-20 Domains & Deployments smoke passed"
