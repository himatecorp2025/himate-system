#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE_JAR="${TMPDIR:-/tmp}/himate-start-09-13-cookies.txt"
BODY="${TMPDIR:-/tmp}/himate-start-09-13-body.json"
rm -f "$COOKIE_JAR" "$BODY"
trap 'rm -f "$COOKIE_JAR" "$BODY"' EXIT

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

printf 'login... '
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"email":"admin@example.com","password":"local-development-password"}'   "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'gateway includes storage and partner runtime... '
gateway_health="$(curl -fsS "$BASE_URL/api/v1/health")"
printf '%s' "$gateway_health" | grep -q '"storage":"ok"'
printf '%s' "$gateway_health" | grep -q '"partner-runtime":"ok"'
echo ok

printf 'create START-09 partner... '
created="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"display_name":"START 09 CI Partner","legal_name":"START 09 CI Partner LLC","brand_name":"START 09 CI","category_id":"cat_006","lifecycle":"PROSPECT","contact_name":"CI Admin","contact_email":"ci-admin@example.com","country":"United States"}'   "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$created" | json_field id)"
test -n "$partner_id"
echo "$partner_id"

printf 'pre-license provisioning plan persists without infrastructure allocation... '
prepared="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' -d "{\"partner_id\":\"$partner_id\",\"system_name\":\"START 09 CI\",\"admin_email\":\"ci-admin@example.com\",\"platform_version\":\"0.3.0-start-09-13\",\"desired_release\":\"0.3.0-start-09-13\",\"environment\":\"STAGING\",\"module_preset\":[\"campaigns_utm\"],\"prepare_only\":true}" "$BASE_URL/api/v1/provisioning/jobs")"
printf '%s' "$prepared" | grep -q '"status":"READY"'
printf '%s' "$prepared" | grep -q '"initial_environment":"STAGING"'
printf '%s' "$prepared" | grep -q '"campaigns_utm"'
pre_db_count="$(docker compose exec -T postgres psql -U himate -d postgres -Atc "SELECT COUNT(*) FROM pg_database WHERE datname='himate_$partner_id'")"
test "$pre_db_count" = "0"
echo ok

printf 'license gate blocks provisioning before payment... '
curl -fsS -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json'   -d '{"lifecycle":"LICENSE_PENDING","reason":"START-09 CI"}'   "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json'   -d '{"lifecycle":"READY_TO_PROVISION","reason":"START-09 unpaid provisioning gate"}'   "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
blocked_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR"   -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner_id\",\"system_name\":\"START 09 CI\",\"admin_email\":\"ci-admin@example.com\",\"platform_version\":\"0.3.0-start-09-13\",\"desired_release\":\"0.3.0-start-09-13\",\"module_preset\":[\"campaigns_utm\"]}"   "$BASE_URL/api/v1/provisioning/jobs")"
test "$blocked_code" = "409"
grep -q 'BLOCKED_LICENSE' "$BODY"
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners/$partner_id" | grep -q '"lifecycle":"READY_TO_PROVISION"'
echo ok

printf 'prepare paid license gate... '
curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json'   -d '{"currency":"USD","activation_fee":13000,"activation_fee_waived":false,"activation_fee_reason":"","base_monthly_fee":250,"annual_increase_percent":10,"price_effective_from":"2026-09-20","service_anchor_date":"2026-09-20","reason":"START-09 CI commercial setup"}'   "$BASE_URL/api/v1/billing/partners/$partner_id/terms" >/dev/null

curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"kind":"PAYMENT_EVIDENCE","name":"START-09 CI receipt","storage_url":"ci://start09/receipt.pdf","note":"Ephemeral CI evidence","mime_type":"application/pdf","sha256":"ci-start09","size_bytes":1}'   "$BASE_URL/api/v1/billing/partners/$partner_id/documents" >/dev/null

curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json'   -d '{"currency":"USD","required_amount":13000,"paid_amount":13000,"payment_date":"2026-09-20","payment_reference":"START09-CI-PAID","verified_by":"ci-smoke","note":"CI verified","waived":false,"waiver_reason":""}'   "$BASE_URL/api/v1/billing/partners/$partner_id/license" | grep -q '"status":"PAID"'
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners/$partner_id" | grep -q '"lifecycle":"READY_TO_PROVISION"'
echo ok

printf 'START-09 provisioning engine... '
provisioned="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner_id\",\"system_name\":\"START 09 CI\",\"admin_email\":\"ci-admin@example.com\",\"platform_version\":\"0.3.0-start-09-13\",\"desired_release\":\"0.3.0-start-09-13\",\"module_preset\":[\"campaigns_utm\"]}"   "$BASE_URL/api/v1/provisioning/jobs")"
printf '%s' "$provisioned" | grep -q '"status":"CONFIGURATION_REQUIRED"'
job_id="$(printf '%s' "$provisioned" | json_field id)"
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners/$partner_id" | grep -q '"lifecycle":"CONFIGURATION"'
echo ok

printf 'idempotent provisioning retry... '
curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/provisioning/jobs/$job_id/run" | grep -q '"status":"CONFIGURATION_REQUIRED"'
db_name="himate_$partner_id"
db_count="$(docker compose exec -T postgres psql -U himate -d postgres -Atc "SELECT COUNT(*) FROM pg_database WHERE datname='$db_name'")"
test "$db_count" = "1"
identity="$(docker compose exec -T postgres psql -U himate -d "$db_name" -Atc "SELECT value FROM partner_core.system_meta WHERE key='partner_id'")"
test "$identity" = "$partner_id"
policy="$(docker compose exec -T postgres psql -U himate -d "$db_name" -Atc "SELECT value FROM partner_core.system_meta WHERE key='template_data_policy'")"
test "$policy" = "STRUCTURE_ONLY_NO_KLAVIERHAUS_BUSINESS_DATA"
admin_invite="$(docker compose exec -T postgres psql -U himate -d "$db_name" -Atc "SELECT COUNT(*) FROM partner_core.admin_invites WHERE email='ci-admin@example.com'")"
test "$admin_invite" = "1"
role_name="${db_name}_app"
echo ok

printf 'START-10 isolated staging environment... '
envs="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/environments?partner_id=$partner_id")"
printf '%s' "$envs" | grep -q '"kind":"STAGING"'
printf '%s' "$envs" | grep -q '"environment_status":"READY"'
printf '%s' "$envs" | grep -q '"deployment_status":"DEPLOYED"'
printf '%s' "$envs" | grep -q '"runtime_status":"OK"'
printf '%s' "$envs" | grep -q '"active_release":"0.3.0-start-09-13"'
env_count="$(printf '%s' "$envs" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["items"]))')"
test "$env_count" = "1"
echo ok

printf 'START-11 connector protocol... '
credential="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"environment":"STAGING"}'   "$BASE_URL/api/v1/connectors/$partner_id/credential")"
connector_token="$(printf '%s' "$credential" | json_field token)"
test -n "$connector_token"
curl -fsS -H "Authorization: Bearer $connector_token" -H 'Content-Type: application/json'   -d '{"version":"0.3.0-start-09-13","health":"OK","modules":{"campaigns_utm":"ACTIVE"}}'   "$BASE_URL/connector/v1/heartbeat" | grep -q '"status":"accepted"'
desired="$(curl -fsS -H "Authorization: Bearer $connector_token" "$BASE_URL/connector/v1/desired-state")"
printf '%s' "$desired" | grep -q '"campaigns_utm":"ACTIVE"'
printf '%s' "$desired" | grep -q '"status":"ACTIVE"'
revision1="$(printf '%s' "$desired" | json_field revision)"
curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json' -d '{"environment":"STAGING","entitlements":{"campaigns_utm":"MAINTENANCE"},"maintenance":{"status":"MAINTENANCE","reason":"CI downstream sync"},"config":{"feature_flag":"ci-enabled"}}' "$BASE_URL/api/v1/connectors/$partner_id/desired-state" >/dev/null
desired2="$(curl -fsS -H "Authorization: Bearer $connector_token" "$BASE_URL/connector/v1/desired-state")"
printf '%s' "$desired2" | grep -q '"campaigns_utm":"MAINTENANCE"'
printf '%s' "$desired2" | grep -q '"status":"MAINTENANCE"'
printf '%s' "$desired2" | grep -q '"feature_flag":"ci-enabled"'
revision2="$(printf '%s' "$desired2" | json_field revision)"
test "$revision2" -gt "$revision1"
echo ok

printf 'START-13 metric definition and connector sync... '
curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"metric_key":"ci.events","label":"CI Events","description":"START-13 smoke metric","unit":"count","aggregation":"SUM","scope":"PARTNER"}'   "$BASE_URL/api/v1/impact/definitions" | grep -q '"metric_key":"ci.events"'
curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner_id\",\"metric_key\":\"ci.events\",\"period_start\":\"2026-08-01\",\"period_end\":\"2026-08-31\",\"numeric_value\":5,\"provenance\":\"MANUAL\",\"source_ref\":\"ci-baseline\"}"   "$BASE_URL/api/v1/impact/baselines" | grep -q '"numeric_value":5'
curl -fsS -H "Authorization: Bearer $connector_token" -H 'Content-Type: application/json'   -d '{"items":[{"idempotency_key":"start09-13-ci-event-1","metric_key":"ci.events","period_start":"2026-09-01","period_end":"2026-09-20","numeric_value":7,"provenance":"PARTNER_DECLARED","source_ref":"ci-connector"}]}'   "$BASE_URL/connector/v1/metrics" | grep -q '"accepted":1'
summary="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/impact/summary?partner_id=$partner_id")"
printf '%s' "$summary" | grep -q '"metric_key":"ci.events"'
printf '%s' "$summary" | grep -q '"numeric_value":7'
printf '%s' "$summary" | grep -q '"baseline_numeric_value":5'
printf '%s' "$summary" | grep -q '"delta_from_baseline":2'
echo ok

printf 'START-12 system health... '
health="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/system-health")"
printf '%s' "$health" | grep -q '"name":"provisioning"'
printf '%s' "$health" | grep -q '"name":"connector"'
printf '%s' "$health" | grep -q '"name":"impact"'
printf '%s' "$health" | grep -q "\"partner_id\":\"$partner_id\""
printf '%s' "$health" | grep -q '"connector_health":"OK"'
printf '%s' "$health" | python3 -c 'import json,sys; p=json.load(sys.stdin); pid=sys.argv[1]; item=next(x for x in p["partners"] if x["partner_id"]==pid); assert item["database_health"]=="OK", item; assert item["storage_health"]=="READY", item; assert item["hostname_status"]=="REACHABLE", item; assert item["sync_status"]=="CURRENT", item' "$partner_id"
echo ok

printf 'cross-tenant database isolation... '
created2="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' -d '{"display_name":"START 09 Isolation Partner","legal_name":"START 09 Isolation Partner LLC","category_id":"cat_006","lifecycle":"PROSPECT","contact_name":"Isolation Admin","contact_email":"isolation@example.com","country":"United States"}' "$BASE_URL/api/v1/partners")"
partner2_id="$(printf '%s' "$created2" | json_field id)"
curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","activation_fee":13000,"activation_fee_waived":false,"base_monthly_fee":250,"annual_increase_percent":10,"price_effective_from":"2026-09-20","service_anchor_date":"2026-09-20","reason":"Isolation CI"}' "$BASE_URL/api/v1/billing/partners/$partner2_id/terms" >/dev/null
curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' -d '{"kind":"PAYMENT_EVIDENCE","name":"Isolation receipt","storage_url":"ci://start09/isolation.pdf","mime_type":"application/pdf","sha256":"ci-isolation","size_bytes":1}' "$BASE_URL/api/v1/billing/partners/$partner2_id/documents" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":13000,"paid_amount":13000,"payment_date":"2026-09-20","payment_reference":"START09-CI-ISO","verified_by":"ci-smoke","waived":false,"waiver_reason":""}' "$BASE_URL/api/v1/billing/partners/$partner2_id/license" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json' -d '{"lifecycle":"LICENSE_PENDING","reason":"Isolation CI"}' "$BASE_URL/api/v1/partners/$partner2_id" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json' -d '{"lifecycle":"READY_TO_PROVISION","reason":"Isolation CI ready"}' "$BASE_URL/api/v1/partners/$partner2_id" >/dev/null
curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' -d "{\"partner_id\":\"$partner2_id\",\"system_name\":\"START 09 Isolation\",\"admin_email\":\"isolation@example.com\",\"platform_version\":\"0.3.0-start-09-13\",\"desired_release\":\"0.3.0-start-09-13\",\"module_preset\":[]}" "$BASE_URL/api/v1/provisioning/jobs" | grep -q '"status":"CONFIGURATION_REQUIRED"'
db2_name="himate_$partner2_id"
role2_name="${db2_name}_app"
a_to_b="$(docker compose exec -T postgres psql -U himate -d postgres -Atc "SELECT has_database_privilege('$role_name','$db2_name','CONNECT')")"
b_to_a="$(docker compose exec -T postgres psql -U himate -d postgres -Atc "SELECT has_database_privilege('$role2_name','$db_name','CONNECT')")"
test "$a_to_b" = "f"
test "$b_to_a" = "f"
echo ok

echo "HIMATE START-09–13 integration smoke passed"
