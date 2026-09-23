#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113h-owner.txt"
BODY="$TMP_ROOT/himate-start23113h-body.json"
rm -f "$OWNER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

STAMP="$(date +%s)"
DISPLAY="Klavierhaus Test Partner $STAMP"
REQ_ONE="qa-onboarding-$STAMP-a"
REQ_TWO="qa-onboarding-$STAMP-b"

partner_payload() {
  python3 - "$DISPLAY" "$1" "$2" "$STAMP" <<'PY'
import json,sys
display,request_id,suffix,stamp=sys.argv[1:]
print(json.dumps({
  "display_name":display,
  "legal_name":display+" LLC",
  "brand_name":display,
  "category_id":"cat_003",
  "lifecycle":"PROSPECT",
  "registration_number":"QA-"+stamp+"-"+suffix,
  "tax_id":"QA-TAX-"+stamp+"-"+suffix,
  "country":"United States",
  "state_region":"New York",
  "city":"New York",
  "postal_code":"10001",
  "address_line1":"1 Test Partner Way",
  "contact_name":"Test Partner Owner",
  "contact_email":"test.partner."+stamp+"."+suffix+"@himate.test",
  "onboarding_request_id":request_id
}))
PY
}

PAYLOAD_ONE="$(partner_payload "$REQ_ONE" a)"
PAYLOAD_TWO="$(partner_payload "$REQ_TWO" b)"

printf 'first partner with a reusable onboarding request ID is created... '
FIRST="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$PAYLOAD_ONE" "$BASE_URL/api/v1/partners")"
FIRST_ID="$(python3 - "$FIRST" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["display_name"], d
assert d["id"], d
assert d["slug"], d
print(d["id"])
PY
)"
echo ok

printf 'replaying the same onboarding request returns the same partner instead of creating a duplicate... '
REPLAY="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$PAYLOAD_ONE" "$BASE_URL/api/v1/partners")"
python3 - "$FIRST" "$REPLAY" <<'PY'
import json,sys
a=json.loads(sys.argv[1]); b=json.loads(sys.argv[2])
assert a["id"]==b["id"], (a,b)
assert a["slug"]==b["slug"], (a,b)
PY
echo ok

printf 'an exact duplicate display name is allowed for a different partner... '
SECOND="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$PAYLOAD_TWO" "$BASE_URL/api/v1/partners")"
python3 - "$FIRST" "$SECOND" "$DISPLAY" <<'PY'
import json,sys
a=json.loads(sys.argv[1]); b=json.loads(sys.argv[2]); display=sys.argv[3]
assert a["display_name"]==display and b["display_name"]==display, (a,b)
assert a["id"]!=b["id"], (a,b)
assert a["slug"]!=b["slug"], (a,b)
PY
echo ok

printf 'Portal Owner state can be reconciled after a committed response is lost... '
OWNER_PAYLOAD="$(python3 - "$STAMP" <<'PY'
import json,sys
stamp=sys.argv[1]
print(json.dumps({
  "name":"Test Partner Owner",
  "email":"owner.retry."+stamp+"@himate.test",
  "password":"RetryOwner!234567",
  "role":"owner"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$OWNER_PAYLOAD" "$BASE_URL/api/v1/partners/$FIRST_ID/portal-users" >/dev/null
USERS="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$FIRST_ID/portal-users")"
python3 - "$USERS" "$STAMP" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); email="owner.retry."+sys.argv[2]+"@himate.test"
matches=[x for x in d["items"] if x["email"].lower()==email.lower() and x["role"]=="owner" and x["active"]]
assert len(matches)==1, d
PY
echo ok

printf 'commercial defaults can be read back and recognized before a retry writes again... '
TODAY="$(date -u +%Y-%m-%d)"
TERMS="$(python3 - "$TODAY" <<'PY'
import json,sys
today=sys.argv[1]
print(json.dumps({
  "currency":"USD",
  "activation_fee":0,
  "activation_fee_waived":False,
  "activation_fee_reason":"",
  "base_monthly_fee":0,
  "minimum_monthly_commitment":1500,
  "quote_reference":"START-23.11.3H-QA",
  "annual_increase_percent":10,
  "price_effective_from":today,
  "service_anchor_date":today,
  "reason":"START-23.11.3h retry reconciliation QA"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$TERMS" "$BASE_URL/api/v1/billing/partners/$FIRST_ID/terms" >/dev/null
CURRENT="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$FIRST_ID/terms")"
python3 - "$CURRENT" "$TODAY" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); today=sys.argv[2]
assert d["currency"]=="USD", d
assert float(d["activation_fee"])==0, d
assert float(d["base_monthly_fee"])==0, d
assert float(d["minimum_monthly_commitment"])==1500, d
assert float(d["annual_increase_percent"])==10, d
assert d["quote_reference"]=="START-23.11.3H-QA", d
assert d["price_effective_from"]==today and d["service_anchor_date"]==today, d
PY
echo ok

echo 'HIMATE START-23.11.3h Test Partner Onboarding Hardening smoke passed'
