#!/usr/bin/env sh

START234_WEBHOOK_SECRET="${START234_WEBHOOK_SECRET:-whsec_local_start234_test}"

provider_success_webhook() {
  base_url="$1"
  attempt_json="$2"
  event_id="$3"
  customer_id="$4"
  payment_method_id="$5"

  parsed="$(printf '%s' "$attempt_json" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["id"]); print(d["provider_payment_id"]); print(int(round(float(d["amount"])*100))); print(str(d["currency"]).lower())')"
  attempt_id="$(printf '%s\n' "$parsed" | sed -n '1p')"
  provider_payment_id="$(printf '%s\n' "$parsed" | sed -n '2p')"
  amount_cents="$(printf '%s\n' "$parsed" | sed -n '3p')"
  currency="$(printf '%s\n' "$parsed" | sed -n '4p')"

  body="$(python3 - "$event_id" "$attempt_id" "$provider_payment_id" "$amount_cents" "$currency" "$customer_id" "$payment_method_id" <<'PY'
import json,sys
event_id,attempt_id,payment_id,amount,currency,customer,method=sys.argv[1:]
print(json.dumps({
  "id":event_id,
  "type":"payment_intent.succeeded",
  "data":{"object":{
    "id":payment_id,
    "status":"succeeded",
    "amount_received":int(amount),
    "currency":currency,
    "customer":customer,
    "payment_method":method,
    "metadata":{"attempt_id":attempt_id},
  }},
},separators=(",",":")))
PY
)"
  timestamp="$(date +%s)"
  signature="$(python3 - "$timestamp" "$START234_WEBHOOK_SECRET" "$body" <<'PY'
import hashlib,hmac,sys
ts,secret,body=sys.argv[1:]
print(hmac.new(secret.encode(),f"{ts}.{body}".encode(),hashlib.sha256).hexdigest())
PY
)"
  curl -fsS -X POST -H 'Content-Type: application/json' -H "Stripe-Signature: t=$timestamp,v1=$signature" -d "$body" "$base_url/webhooks/stripe"
}

provider_pay_activation() {
  base_url="$1"
  cookie="$2"
  partner_id="$3"
  suffix="$4"
  customer="cus_ci_$suffix"
  method="pm_ci_$suffix"

  curl -fsS -b "$cookie" -X PUT -H 'Content-Type: application/json'     -d "{"provider_customer_id":"$customer","payment_method_id":"$method","autopay_enabled":true}"     "$base_url/api/v1/payments/partners/$partner_id/profile" >/dev/null
  attempt="$(curl -fsS -b "$cookie" -X POST -H 'Content-Type: application/json' -d '{}'     "$base_url/api/v1/billing/partners/$partner_id/license/collect")"
  provider_success_webhook "$base_url" "$attempt" "evt_activation_$suffix" "$customer" "$method" >/dev/null
}
