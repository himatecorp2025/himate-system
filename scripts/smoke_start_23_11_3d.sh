#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113d-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-start23113d-partner.txt"
BODY="$TMP_ROOT/himate-start23113d-body.json"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

owner_login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$owner_login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'legacy fixed TEST partner and portal identity are retired... '
docker compose exec -T postgres sh -lc 'psql -At -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<SQL
SELECT COUNT(*) FROM partners.partners WHERE id='"'"'ptr_himate_test_001'"'"' OR slug='"'"'himate-test-partner'"'"' OR lower(contact_email)='"'"'test.partner@himate.test'"'"';
SELECT COUNT(*) FROM identity.partner_users WHERE id='"'"'pusr_himate_test_001'"'"' OR partner_id='"'"'ptr_himate_test_001'"'"' OR lower(email)='"'"'test.partner@himate.test'"'"';
SQL' | python3 -c 'import sys; values=[int(x.strip()) for x in sys.stdin if x.strip()]; assert values==[0,0], values'
echo ok

STAMP="$(date +%s)"
DISPLAY_NAME="Portal Onboarding QA $STAMP"
EMAIL="portal.onboarding.$STAMP@himate.test"
PASSWORD="PortalOnboarding!2345Aa"

partner_payload="$(python3 - "$DISPLAY_NAME" "$EMAIL" <<'PY'
import json,sys
print(json.dumps({
  "display_name":sys.argv[1],
  "legal_name":sys.argv[1]+" LLC",
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "contact_name":"Portal QA Owner",
  "contact_email":sys.argv[2],
  "country":"United States",
  "primary_domain":""
}))
PY
)"

printf 'normal HIMATE admin flow can create the partner record... '
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
PARTNER_ID="$(printf '%s' "$partner" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle"]=="PROSPECT"; print(d["id"])')"
test -n "$PARTNER_ID"
echo ok

portal_payload="$(python3 - "$EMAIL" "$PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Portal QA Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"

printf 'the same admin onboarding flow can register the first Partner Portal Owner... '
portal_user="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_payload" "$BASE_URL/api/v1/partners/$PARTNER_ID/portal-users")"
printf '%s' "$portal_user" | python3 - "$PARTNER_ID" "$EMAIL" <<'PY'
import json,sys
d=json.load(sys.stdin)
assert d["partner_id"]==sys.argv[1]
assert d["email"]==sys.argv[2]
assert d["role"]=="owner"
assert d["active"] is True
PY
echo ok

printf 'the newly registered partner can immediately authenticate at Partner Portal... '
login_payload="$(python3 - "$EMAIL" "$PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
login_response="$(curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/partner/api/v1/auth/login")"
printf '%s' "$login_response" | python3 - "$PARTNER_ID" "$EMAIL" <<'PY'
import json,sys
d=json.load(sys.stdin)
assert d["partner_id"]==sys.argv[1]
assert d["email"]==sys.argv[2]
assert d["role"]=="owner"
PY
curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/auth/me" >/dev/null
echo ok

echo 'HIMATE START-23.11.3d partner onboarding smoke passed'
