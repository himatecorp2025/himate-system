#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE_JAR="${TMPDIR:-/tmp}/himate-smoke-cookies.txt"
rm -f "$COOKIE_JAR"
trap 'rm -f "$COOKIE_JAR"' EXIT

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

status_code() {
  curl -sS -o "${TMPDIR:-/tmp}/himate-smoke-body.json" -w '%{http_code}' "$@"
}

printf 'health... '
curl -fsS "$BASE_URL/api/v1/health" | grep -q '"status":"ok"'
echo ok

printf 'unauthorized boundary... '
code="$(status_code "$BASE_URL/api/v1/partners")"
test "$code" = "401"
echo ok

printf 'login... '
curl -fsS -c "$COOKIE_JAR"   -H 'Content-Type: application/json'   -d '{"email":"admin@example.com","password":"Local-Development1!Password"}'   "$BASE_URL/api/v1/auth/login" >/dev/null
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/auth/me" | grep -q 'admin@example.com'
echo ok

printf 'reference partner portfolio... '
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners?limit=5&offset=0" | grep -q 'ptr_000001'
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners/ptr_000001" | grep -q '"lifecycle":"LIVE"'
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners/ptr_000001/modules" | grep -q 'workshop_workflow'
echo ok

printf 'billing foundation... '
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/billing/profile" | grep -q '"legal_name"'
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/billing/partners/ptr_000001/terms" | grep -q '"base_monthly_fee":2000'
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/billing/partners/ptr_000001/license" | grep -q '"status":"WAIVED"'
echo ok

printf 'create prospect... '
created="$(curl -fsS -b "$COOKIE_JAR"   -H 'Content-Type: application/json'   -d '{"display_name":"CI Smoke Partner","legal_name":"CI Smoke Partner LLC","category_id":"cat_006","lifecycle":"PROSPECT","country":"United States"}'   "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$created" | json_field id)"
test -n "$partner_id"
echo "$partner_id"

printf 'reject invalid lifecycle jump... '
code="$(status_code -b "$COOKIE_JAR" -X PATCH   -H 'Content-Type: application/json'   -d '{"lifecycle":"LIVE","reason":"CI invalid jump test"}'   "$BASE_URL/api/v1/partners/$partner_id")"
test "$code" = "409"
echo ok

printf 'license gate fail-closed... '
curl -fsS -b "$COOKIE_JAR" -X PATCH   -H 'Content-Type: application/json'   -d '{"lifecycle":"LICENSE_PENDING","reason":"CI lifecycle test"}'   "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X PATCH   -H 'Content-Type: application/json'   -d '{"lifecycle":"READY_TO_PROVISION","reason":"CI lifecycle test"}'   "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
code="$(status_code -b "$COOKIE_JAR" -X PATCH   -H 'Content-Type: application/json'   -d '{"lifecycle":"PROVISIONING","reason":"CI unpaid provisioning test"}'   "$BASE_URL/api/v1/partners/$partner_id")"
test "$code" = "409"
echo ok

printf 'license evidence and paid gate... '
curl -fsS -b "$COOKIE_JAR" -X PUT   -H 'Content-Type: application/json'   -d '{"currency":"USD","activation_fee":13000,"activation_fee_waived":false,"activation_fee_reason":"","base_monthly_fee":250,"annual_increase_percent":10,"price_effective_from":"2026-09-20","service_anchor_date":"2026-09-20","reason":"CI commercial setup"}'   "$BASE_URL/api/v1/billing/partners/$partner_id/terms" >/dev/null
curl -fsS -b "$COOKIE_JAR"   -H 'Content-Type: application/json'   -d '{"kind":"PAYMENT_EVIDENCE","name":"CI receipt","storage_url":"ci://receipt/paid.pdf","note":"Ephemeral CI evidence","mime_type":"application/pdf","sha256":"ci-smoke","size_bytes":1}'   "$BASE_URL/api/v1/billing/partners/$partner_id/documents" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X PUT   -H 'Content-Type: application/json'   -d '{"currency":"USD","required_amount":13000,"paid_amount":13000,"payment_date":"2026-09-20","payment_reference":"CI-PAID-001","verified_by":"ci-smoke","note":"CI verified","waived":false,"waiver_reason":""}'   "$BASE_URL/api/v1/billing/partners/$partner_id/license" | grep -q '"status":"PAID"'
curl -fsS -b "$COOKIE_JAR" -X PATCH   -H 'Content-Type: application/json'   -d '{"lifecycle":"PROVISIONING","reason":"CI paid provisioning gate test"}'   "$BASE_URL/api/v1/partners/$partner_id" | grep -q '"lifecycle":"PROVISIONING"'
echo ok

printf 'module entitlement and pricing... '
curl -fsS -b "$COOKIE_JAR" -X PATCH   -H 'Content-Type: application/json'   -d '{"status":"ACTIVE","visible":true,"included_in_base":false,"partner_price":49,"reason":"CI module activation"}'   "$BASE_URL/api/v1/partners/$partner_id/modules/campaigns_utm" | grep -q '"status":"ACTIVE"'
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/billing/partners/$partner_id/summary" | grep -q '"cycle_days":30'
echo ok

echo "HIMATE START-01–08 backend smoke passed"
