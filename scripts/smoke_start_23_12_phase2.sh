#!/usr/bin/env sh
set -eu
BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-2312-p2-owner.txt"
rm -f "$OWNER_COOKIE"
trap 'rm -f "$OWNER_COOKIE"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
TODAY="$(date -u +%Y-%m-%d)"
REQ_ID="phase2-onboarding-$STAMP"
PORTAL_EMAIL="phase2-owner-$STAMP@himate.test"
PORTAL_PASSWORD="Phase2!DurableOwner-$STAMP"

LOGIN="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$LOGIN" "$BASE_URL/api/v1/auth/login" >/dev/null

PAYLOAD="$(python3 - "$REQ_ID" "$PORTAL_EMAIL" "$PORTAL_PASSWORD" "$STAMP" "$TODAY" <<'PY'
import json,sys
req,email,password,stamp,today=sys.argv[1:]
print(json.dumps({
 "request_id":req,
 "partner":{
  "display_name":"Phase 2 Durable Partner "+stamp,
  "legal_name":"Phase 2 Durable Partner LLC "+stamp,
  "category_id":"cat_005","lifecycle":"PROSPECT",
  "contact_name":"Phase 2 Owner","contact_email":email,
  "registration_number":"P2-"+stamp,"tax_id":"P2-TAX-"+stamp,
  "country":"United States","city":"New York","postal_code":"10001","address_line1":"1 Phase 2 Way"
 },
 "portal_owner":{"name":"Phase 2 Owner","email":email,"password":password},
 "billing_terms":{
  "currency":"USD","activation_fee":0,"activation_fee_waived":True,
  "activation_fee_reason":"Phase 2 acceptance","base_monthly_fee":1500,
  "minimum_monthly_commitment":1500,"quote_reference":"P2-"+stamp,
  "annual_increase_percent":10,"price_effective_from":today,"service_anchor_date":today,
  "reason":"START-23.12 Phase 2 durable onboarding"
 }
}))
PY
)"
FIRST="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$PAYLOAD" "$BASE_URL/api/v1/partner-onboarding")"
SECOND="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$PAYLOAD" "$BASE_URL/api/v1/partner-onboarding")"
python3 - "$FIRST" "$SECOND" <<'PY'
import json,sys
a,b=map(json.loads,sys.argv[1:3])
assert a["status"]=="COMPLETE" and b["status"]=="COMPLETE",(a,b)
assert a["partner"]["id"]==b["partner"]["id"],(a,b)
assert a["owner_done"] and a["billing_done"],a
print("durable onboarding replay... ok")
PY

curl -fsS "$BASE_URL/partner/app/billing" | grep -q 'flutter_bootstrap.js'
echo 'Partner Portal deep-link reload... ok'
echo 'HIMATE START-23.12 Phase 2 onboarding smoke passed'
