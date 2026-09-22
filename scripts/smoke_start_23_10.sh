#!/usr/bin/env sh
set -eu

. scripts/payment_test_helpers.sh
. scripts/evidence_test_helpers.sh

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start2310-owner.txt"
BODY="$TMP_ROOT/himate-start2310-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
TODAY="$(date -u +%Y-%m-%d)"
DOMAIN="start2310-$STAMP.example.com"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-23.10 owner login... '
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'create commercially governed provisioning partner... '
partner_payload="$(python3 - "$STAMP" "$DOMAIN" <<'PY'
import json,sys
s,domain=sys.argv[1:]
print(json.dumps({
 "display_name":"START 23.10 Operations "+s,
 "legal_name":"START 23.10 Operations LLC "+s,
 "brand_name":"START2310 "+s,
 "primary_domain":domain,
 "contact_name":"Operations Owner",
 "contact_email":"start2310-"+s+"@example.com",
 "country":"US"
}))
PY
)"
partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"

terms="$(python3 - "$TODAY" <<'PY'
import json,sys
day=sys.argv[1]
print(json.dumps({
 "currency":"USD","activation_fee":13000,"activation_fee_waived":False,"activation_fee_reason":"",
 "base_monthly_fee":125,"annual_increase_percent":0,
 "price_effective_from":day,"service_anchor_date":day,
 "reason":"START-23.10 System & Operations acceptance"
}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$terms" "$BASE_URL/api/v1/billing/partners/$partner_id/terms" >/dev/null
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json'   -d '{"status":"AGREED","agreement_reference":"contract://start2310/signed","note":"START-23.10 acceptance"}'   "$BASE_URL/api/v1/billing/partners/$partner_id/agreement" >/dev/null
register_billing_evidence_document "$COOKIE" "$partner_id" "INVOICE" "INVOICE" "START-23.10 activation invoice" "start2310-invoice-$STAMP" >/dev/null
register_billing_evidence_document "$COOKIE" "$partner_id" "PAYMENT_EVIDENCE" "OTHER" "START-23.10 payment evidence" "start2310-payment-$STAMP" >/dev/null
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json'   -d '{"currency":"USD","required_amount":13000,"note":"START-23.10 provider-backed activation","waived":false,"waiver_reason":""}'   "$BASE_URL/api/v1/billing/partners/$partner_id/license" >/dev/null
provider_pay_activation "$BASE_URL" "$COOKIE" "$partner_id" "start2310_$STAMP"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"lifecycle":"LICENSE_PENDING","reason":"START-23.10 commercial preparation"}' "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"lifecycle":"READY_TO_PROVISION","reason":"START-23.10 commercial gate satisfied"}' "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/commercial-status" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["provisioning_allowed"] is True,d'
echo ok

printf 'prepare, execute and idempotently retry Provisioning Engine... '
provision_payload="$(python3 - "$partner_id" "$STAMP" <<'PY'
import json,sys
p,s=sys.argv[1:]
print(json.dumps({
 "partner_id":p,"system_name":"START2310-"+s,
 "admin_email":"ops-"+s+"@example.com",
 "platform_version":"23.10.0","desired_release":"start-23.10-ci",
 "environment":"STAGING","module_preset":[],"prepare_only":True
}))
PY
)"
prepared="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$provision_payload" "$BASE_URL/api/v1/provisioning/jobs")"
job_id="$(printf '%s' "$prepared" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="READY",d; print(d["id"])')"
executed="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/provisioning/jobs/$job_id/run")"
printf '%s' "$executed" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="CONFIGURATION_REQUIRED",d; assert d["current_step"]=="COMPLETE",d; assert d["steps"] and all(x["status"]=="SUCCESS" for x in d["steps"]),d'
attempts_before="$(printf '%s' "$executed" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(",".join(str(x["attempts"]) for x in d["steps"]))')"
retried="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/provisioning/jobs/$job_id/run")"
printf '%s' "$retried" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="CONFIGURATION_REQUIRED"; assert ",".join(str(x["attempts"]) for x in d["steps"])==sys.argv[1],(sys.argv[1],d)' "$attempts_before"
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id" | grep -q '"lifecycle":"CONFIGURATION"'
echo ok

printf 'connector credential authenticates, rotates and invalidates old raw token... '
cred1="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"environment":"STAGING"}' "$BASE_URL/api/v1/connectors/$partner_id/credential")"
token1="$(printf '%s' "$cred1" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["token_returned_once"] is True; print(d["token"])')"
curl -fsS -H "Authorization: Bearer $token1" "$BASE_URL/connector/v1/state" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1] and d["environment"]=="STAGING"' "$partner_id"
cred2="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"environment":"STAGING"}' "$BASE_URL/api/v1/connectors/$partner_id/credential")"
token2="$(printf '%s' "$cred2" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')"
test "$token1" != "$token2"
test "$(curl -sS -o "$BODY" -w '%{http_code}' -H "Authorization: Bearer $token1" "$BASE_URL/connector/v1/state")" = "401"
curl -fsS -H "Authorization: Bearer $token2" "$BASE_URL/connector/v1/state" >/dev/null
credential_meta="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/connectors/$partner_id/credential")"
printf '%s' "$credential_meta" | python3 -c 'import json,sys; raw=sys.stdin.read(); d=json.loads(raw); assert "token_hash" not in raw and "\"token\"" not in raw; assert d["items"]'
echo ok

printf 'Website Adapter fails closed before correct domain binding, then succeeds... '
prod_cred="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"environment":"PRODUCTION"}' "$BASE_URL/api/v1/connectors/$partner_id/credential")"
prod_token="$(printf '%s' "$prod_cred" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')"
bad_adapter='{"environment":"PRODUCTION","adapter_type":"GENERIC_HTTP","site_base_url":"https://wrong.example.net","allowed_domains":["wrong.example.net"],"capabilities":["ENTITLEMENTS","HEARTBEAT","METRICS","AGGREGATED_DATA","RECONCILIATION"],"enabled":true,"config":{}}'
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$bad_adapter" "$BASE_URL/api/v1/connectors/$partner_id/website-adapter" >/dev/null
test "$(curl -sS -o "$BODY" -w '%{http_code}' -H "Authorization: Bearer $prod_token" "$BASE_URL/connector/v1/commercial-state")" = "409"
grep -q 'DOMAIN_BINDING_MISMATCH' "$BODY"
good_adapter="$(python3 - "$DOMAIN" <<'PY'
import json,sys
domain=sys.argv[1]
print(json.dumps({"environment":"PRODUCTION","adapter_type":"GENERIC_HTTP","site_base_url":"https://"+domain,"allowed_domains":[domain],"capabilities":["ENTITLEMENTS","HEARTBEAT","METRICS","AGGREGATED_DATA","RECONCILIATION"],"enabled":True,"config":{"integration":"existing-site"}}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$good_adapter" "$BASE_URL/api/v1/connectors/$partner_id/website-adapter" >/dev/null
curl -fsS -H "Authorization: Bearer $prod_token" "$BASE_URL/connector/v1/commercial-state" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]; assert d["tenant_scope"]=="CREDENTIAL_BOUND"; assert d["domain_binding"]["primary_domain"]==sys.argv[2]' "$partner_id" "$DOMAIN"
echo ok

printf 'environment create, edit, deploy/retry and DNS/TLS launch gates persist... '
prod_payload="$(python3 - "$partner_id" "$DOMAIN" <<'PY'
import json,sys
p,domain=sys.argv[1:]
print(json.dumps({"partner_id":p,"kind":"PRODUCTION","hostname":domain,"platform_version":"23.10.0","desired_release":"start-23.10-prod","config":{"source":"start-23.10"}}))
PY
)"
prod="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$prod_payload" "$BASE_URL/api/v1/environments")"
prod_id="$(printf '%s' "$prod" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["kind"]=="PRODUCTION"; print(d["id"])')"
edited="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"platform_version":"23.10.1","desired_release":"start-23.10-prod-r1"}' "$BASE_URL/api/v1/environments/$prod_id")"
printf '%s' "$edited" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["platform_version"]=="23.10.1"; assert d["desired_release"]=="start-23.10-prod-r1"'
deploy1="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{"release":"start-23.10-prod-r1"}' "$BASE_URL/api/v1/environments/$prod_id/deploy")"
printf '%s' "$deploy1" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["deployment_status"]=="DEPLOYED"; assert d["active_release"]=="start-23.10-prod-r1"; assert d["runtime_status"]=="OK"'
deploy2="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{"release":"start-23.10-prod-r2"}' "$BASE_URL/api/v1/environments/$prod_id/deploy")"
printf '%s' "$deploy2" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["deployment_status"]=="DEPLOYED"; assert d["active_release"]=="start-23.10-prod-r2"'
provider="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT provider||':'||status||':'||provider_deploy_id FROM runtime.deployments WHERE partner_id='$partner_id' AND environment='PRODUCTION' ORDER BY updated_at DESC LIMIT 1")"
printf '%s' "$provider" | grep -q '^local:READY:local_'
verified="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/environments/$prod_id/verify-domain")"
printf '%s' "$verified" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["domain_status"]=="FAILED"; assert d["dns_status"]=="FAILED"; assert d["launch_ready"] is False'
test "$(status "$COOKIE" POST "/api/v1/environments/$prod_id/launch" -H 'Content-Type: application/json' -d '{}')" = "409"
grep -q 'LAUNCH_GATE' "$BODY"
echo ok

printf 'backup policy deterministically drives the real scheduler... '
policy="$(curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"retention_days":31,"max_restore_points":7,"schedule_hours":24,"enabled":true}' "$BASE_URL/api/v1/backups/policies/$partner_id")"
printf '%s' "$policy" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["retention_days"]==31 and d["max_restore_points"]==7 and d["schedule_hours"]==24 and d["enabled"] is True'
scheduled="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/backups/scheduler/run")"
printf '%s' "$scheduled" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="ok"; assert int(d["queued"])>=1,d'
policy_after="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/backups/policies/$partner_id")"
printf '%s' "$policy_after" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["last_scheduled_at"],d'
restore_points="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/backups?partner_id=$partner_id")"
printf '%s' "$restore_points" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["items"]; assert any(x["created_by"]=="scheduler" for x in d["items"]),d'
echo ok

printf '23.10 operations mutations reached central audit... '
sleep 1
audit="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/audit/events?limit=200")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); p=sys.argv[1]; rows=[x for x in d["items"] if x.get("partner_id")==p or p in x.get("path","")]; resources={x["resource"] for x in rows if x["outcome"]=="SUCCESS"}; assert {"provisioning","connectors","environments"} <= resources,(resources,rows)' "$partner_id"
echo ok

echo "HIMATE START-23.10 System & Operations Production Closure smoke passed"
