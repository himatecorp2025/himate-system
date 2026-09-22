#!/usr/bin/env sh
set -eu

. scripts/payment_test_helpers.sh
. scripts/evidence_test_helpers.sh

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start234-owner.txt"
BODY="$TMP_ROOT/himate-start234-body.json"
rm -f "$OWNER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
TODAY="$(date -u +%Y-%m-%d)"
PREV30="$(python3 - "$TODAY" <<'PY'
from datetime import date,timedelta
import sys
print((date.fromisoformat(sys.argv[1])-timedelta(days=30)).isoformat())
PY
)"
NEXT_MONTH="$(python3 - "$TODAY" <<'PY'
from datetime import date
import sys
d=date.fromisoformat(sys.argv[1])
print(date(d.year+1,1,1).isoformat() if d.month==12 else date(d.year,d.month+1,1).isoformat())
PY
)"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-23.4 owner login... '
payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'create payment-controlled partner and commercial state... '
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.4 Payment Partner","legal_name":"START 23.4 Payment Partner LLC","brand_name":"START 23.4","contact_name":"Payment Owner","contact_email":"payments@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
terms="$(python3 - "$PREV30" <<'PY'
import json,sys
print(json.dumps({
  "currency":"USD","activation_fee":13000,"activation_fee_waived":False,"activation_fee_reason":"",
  "base_monthly_fee":125,"annual_increase_percent":10,
  "price_effective_from":sys.argv[1],"service_anchor_date":sys.argv[1],
  "reason":"START-23.4 provider-backed acceptance"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$terms" "$BASE_URL/api/v1/billing/partners/$partner_id/terms" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"status":"AGREED","agreement_reference":"contract://start234/signed","note":"START-23.4 acceptance"}' "$BASE_URL/api/v1/billing/partners/$partner_id/agreement" >/dev/null
register_billing_evidence_document "$OWNER_COOKIE" "$partner_id" "INVOICE" "INVOICE" "Activation invoice" "start234-invoice" >/dev/null
register_billing_evidence_document "$OWNER_COOKIE" "$partner_id" "PAYMENT_EVIDENCE" "OTHER" "Provider payment evidence" "start234-payment" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":13000,"note":"Provider-backed activation license","waived":false,"waiver_reason":""}' "$BASE_URL/api/v1/billing/partners/$partner_id/license" >/dev/null
echo ok

printf 'manual PAID transition is rejected... '
test "$(status "$OWNER_COOKIE" PUT "/api/v1/billing/partners/$partner_id/license" -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":13000,"paid_amount":13000,"payment_date":"'"$TODAY"'","payment_reference":"MANUAL-BYPASS","verified_by":"ci","waived":false}')" = "409"
grep -q 'PROVIDER_MANAGED_PAYMENT' "$BODY"
echo ok

printf 'configure provider profile and initiate activation charge idempotently... '
customer="cus_start234_$STAMP"
method="pm_start234_$STAMP"
profile_payload="$(python3 - "$customer" "$method" <<'PY'
import json,sys
print(json.dumps({"provider_customer_id":sys.argv[1],"payment_method_id":sys.argv[2],"autopay_enabled":True},separators=(",",":")))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$profile_payload" "$BASE_URL/api/v1/payments/partners/$partner_id/profile" >/dev/null
attempt1="$(curl -fsS -b "$OWNER_COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/billing/partners/$partner_id/license/collect")"
attempt2="$(curl -fsS -b "$OWNER_COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/billing/partners/$partner_id/license/collect")"
printf '%s\n%s' "$attempt1" "$attempt2" | python3 -c 'import json,sys; lines=sys.stdin.read().splitlines(); a=json.loads(lines[0]); b=json.loads(lines[1]); assert a["id"]==b["id"],(a,b); assert a["status"]=="PROCESSING",a; assert a["purpose"]=="ACTIVATION_LICENSE",a'
echo ok

printf 'invalid and amount-mismatched signed webhooks fail closed... '
test "$(curl -sS -o "$BODY" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H 'Stripe-Signature: t=1,v1=deadbeef' -d '{}' "$BASE_URL/webhooks/stripe")" = "400"
grep -q 'INVALID_SIGNATURE' "$BODY"
parsed="$(printf '%s' "$attempt1" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["id"]); print(d["provider_payment_id"])')"
activation_attempt_id="$(printf '%s\n' "$parsed" | sed -n '1p')"
activation_provider_id="$(printf '%s\n' "$parsed" | sed -n '2p')"
bad_body="$(python3 - "$activation_attempt_id" "$activation_provider_id" "$customer" "$method" <<'PY'
import json,sys
attempt,payment,customer,method=sys.argv[1:]
print(json.dumps({"id":"evt_start234_bad_amount","type":"payment_intent.succeeded","data":{"object":{"id":payment,"status":"succeeded","amount_received":1299999,"currency":"usd","customer":customer,"payment_method":method,"metadata":{"attempt_id":attempt}}}},separators=(",",":")))
PY
)"
ts="$(date +%s)"
bad_sig="$(python3 - "$ts" "$START234_WEBHOOK_SECRET" "$bad_body" <<'PY'
import hashlib,hmac,sys
ts,secret,body=sys.argv[1:]
print(hmac.new(secret.encode(),f"{ts}.{body}".encode(),hashlib.sha256).hexdigest())
PY
)"
test "$(curl -sS -o "$BODY" -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H "Stripe-Signature: t=$ts,v1=$bad_sig" -d "$bad_body" "$BASE_URL/webhooks/stripe")" = "409"
grep -q 'PAYMENT_AMOUNT_MISMATCH' "$BODY"
echo ok

printf 'signed provider webhook is the only PAID authority... '
provider_success_webhook "$BASE_URL" "$attempt1" "evt_start234_activation_$STAMP" "$customer" "$method" >/dev/null
license="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/license")"
printf '%s' "$license" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="PAID",d; assert float(d["paid_amount"])==13000,d; assert str(d["payment_reference"]).startswith("pi_mock_"),d; assert d["verified_by"]=="payments-service",d'
ready="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/commercial-status")"
printf '%s' "$ready" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["provisioning_allowed"] is True,d'
echo ok

printf 'calendar-month billing cycle creates recurring invoice and automatic charge attempt... '
docker compose exec -T billing /app/service --run-invoice-cycle "$NEXT_MONTH"
invoices="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/invoices")"
invoice_id="$(printf '%s' "$invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next(i for i in d["items"] if abs(float(i["total"])-1500)<0.01); assert x["billing_model"]=="CALENDAR_MONTH",x; assert x["minimum_commitment_adjustment"]==1375,x; assert x["status"]!="PAID",x; print(x["id"])')"
attempts="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/payments/partners/$partner_id/attempts")"
invoice_attempt="$(printf '%s' "$attempts" | python3 -c 'import json,sys; d=json.load(sys.stdin); invoice=sys.argv[1]; x=next(i for i in d["items"] if i["purpose"]=="INVOICE" and i["invoice_id"]==invoice); assert x["status"]=="PROCESSING",x; print(json.dumps(x,separators=(",",":")))' "$invoice_id")"
echo ok

printf 'recurring invoice settles PAID from signed webhook and duplicate delivery is idempotent... '
event_id="evt_start234_invoice_$STAMP"
provider_success_webhook "$BASE_URL" "$invoice_attempt" "$event_id" "$customer" "$method" >/dev/null
duplicate="$(provider_success_webhook "$BASE_URL" "$invoice_attempt" "$event_id" "$customer" "$method")"
printf '%s' "$duplicate" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="duplicate",d'
invoices="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/invoices")"
printf '%s' "$invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); invoice=sys.argv[1]; x=next(i for i in d["items"] if i["id"]==invoice); assert x["status"]=="PAID",x; assert x["provider_status"]=="SUCCEEDED",x' "$invoice_id"
echo ok

printf 'payment attempts and immutable billing events retain reconciliation evidence... '
attempts="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/payments/partners/$partner_id/attempts")"
printf '%s' "$attempts" | python3 -c 'import json,sys; d=json.load(sys.stdin); purposes={x["purpose"] for x in d["items"]}; statuses={x["status"] for x in d["items"]}; assert {"ACTIVATION_LICENSE","INVOICE"} <= purposes,d; assert "SUCCEEDED" in statuses,d'
events="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/events")"
printf '%s' "$events" | python3 -c 'import json,sys; d=json.load(sys.stdin); types={x["event_type"] for x in d["items"]}; assert {"LICENSE_PAID","INVOICE_GENERATED","INVOICE_PAID"} <= types,types'
echo ok

echo "HIMATE START-23.4 provider-backed activation and recurring autopay smoke passed"
