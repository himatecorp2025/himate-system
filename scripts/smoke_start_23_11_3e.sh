#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113e-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-start23113e-partner.txt"
LOGO_FILE="$TMP_ROOT/himate-start23113e-logo.png"
LOGO_BODY="$TMP_ROOT/himate-start23113e-logo-body.bin"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$LOGO_FILE" "$LOGO_BODY"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$LOGO_FILE" "$LOGO_BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

owner_login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$owner_login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

STAMP="$(date +%s)"
DISPLAY_NAME="HIMATE Fake Test Partner $STAMP"
LEGAL_NAME="HIMATE Fake Test Partner $STAMP LLC"
EMAIL="fake.partner.$STAMP@himate.test"
PASSWORD="FakePartner!2345Aa"
REGISTRATION="FAKE-REG-$STAMP"
TAX_ID="FAKE-TAX-$STAMP"
ADDRESS1="123 Test Avenue"
CITY="Test City"
POSTAL="10001"

partner_payload="$(python3 - "$DISPLAY_NAME" "$LEGAL_NAME" "$EMAIL" "$REGISTRATION" "$TAX_ID" "$ADDRESS1" "$CITY" "$POSTAL" <<'PY'
import json,sys
print(json.dumps({
  "display_name":sys.argv[1],
  "legal_name":sys.argv[2],
  "brand_name":"Fake Brand QA",
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "registration_number":sys.argv[4],
  "tax_id":sys.argv[5],
  "country":"United States",
  "state_region":"New York",
  "city":sys.argv[7],
  "postal_code":sys.argv[8],
  "address_line1":sys.argv[6],
  "address_line2":"Suite QA",
  "website":"https://example.test",
  "phone":"+1-555-0100",
  "primary_domain":"",
  "contact_name":"Fake Partner Owner",
  "contact_email":sys.argv[3],
  "finance_contact_name":"Fake Finance",
  "finance_contact_email":"finance."+sys.argv[3],
  "technical_contact_name":"Fake Technical",
  "technical_contact_email":"technical."+sys.argv[3],
  "marketing_contact_name":"Fake Marketing",
  "marketing_contact_email":"marketing."+sys.argv[3],
  "notes":"Synthetic START-23.11.3e onboarding record"
}))
PY
)"

printf 'complete partner master data can be created through the normal HIMATE API... '
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
PARTNER_ID="$(python3 - "$partner" "$REGISTRATION" "$TAX_ID" "$ADDRESS1" "$CITY" "$POSTAL" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["lifecycle"]=="PROSPECT"
assert d["registration_number"]==sys.argv[2]
assert d["tax_id"]==sys.argv[3]
assert d["address_line1"]==sys.argv[4]
assert d["city"]==sys.argv[5]
assert d["postal_code"]==sys.argv[6]
assert d["brand_name"]=="Fake Brand QA"
assert d["finance_contact_name"]=="Fake Finance"
assert d["technical_contact_name"]=="Fake Technical"
assert d["marketing_contact_name"]=="Fake Marketing"
print(d["id"])
PY
)"
test -n "$PARTNER_ID"
echo ok

portal_payload="$(python3 - "$EMAIL" "$PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Fake Partner Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
printf 'the first Partner Portal Owner can be attached to the newly created partner... '
portal_user="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_payload" "$BASE_URL/api/v1/partners/$PARTNER_ID/portal-users")"
python3 - "$portal_user" "$PARTNER_ID" "$EMAIL" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["partner_id"]==sys.argv[2]
assert d["email"]==sys.argv[3]
assert d["role"]=="owner"
assert d["active"] is True
PY
echo ok

# Small valid 1x1 PNG, used only inside the ephemeral CI database/storage topology.
printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=' | base64 -d > "$LOGO_FILE"

printf 'a partner-scoped logo can be uploaded and linked during onboarding... '
logo_response="$(curl -fsS -b "$OWNER_COOKIE"   -F "file=@$LOGO_FILE;type=image/png"   -F "purpose=logo"   -F "alt_text=$DISPLAY_NAME logo"   "$BASE_URL/api/v1/partners/$PARTNER_ID/logo")"
LOGO_URL="$(python3 - "$logo_response" "$PARTNER_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["partner_id"]==sys.argv[2]
assert d["media"]["purpose"]=="logo"
assert d["media_id"]
assert d["logo_url"].startswith("/public/v1/cms/media/")
assert d["partner"]["logo_url"]==d["logo_url"]
print(d["logo_url"])
PY
)"
echo ok

printf 'the assigned partner logo is publicly readable for branded surfaces... '
STATUS="$(curl -sS -o "$LOGO_BODY" -w '%{http_code}' "$BASE_URL$LOGO_URL")"
test "$STATUS" = "200"
test -s "$LOGO_BODY"
echo ok

printf 'the partner master record retains the complete billing/company identity and logo... '
stored="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$PARTNER_ID")"
python3 - "$stored" "$REGISTRATION" "$TAX_ID" "$LOGO_URL" "$ADDRESS1" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["registration_number"]==sys.argv[2]
assert d["tax_id"]==sys.argv[3]
assert d["logo_url"]==sys.argv[4]
assert d["address_line1"]==sys.argv[5]
assert d["country"]=="United States"
assert d["finance_contact_email"].startswith("finance.")
assert d["technical_contact_email"].startswith("technical.")
assert d["marketing_contact_email"].startswith("marketing.")
PY
echo ok

printf 'the newly onboarded owner can sign in and read the same company identity from Partner Portal... '
login_payload="$(python3 - "$EMAIL" "$PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
company="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/company")"
python3 - "$company" "$REGISTRATION" "$TAX_ID" "$LOGO_URL" "$LEGAL_NAME" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["registration_number"]==sys.argv[2]
assert d["tax_id"]==sys.argv[3]
assert d["logo_url"]==sys.argv[4]
assert d["legal_name"]==sys.argv[5]
PY
echo ok

echo 'HIMATE START-23.11.3e New Partner master-data onboarding smoke passed'
