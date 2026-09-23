#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113g-owner.txt"
BODY="$TMP_ROOT/himate-start23113g-body.json"
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

printf 'unknown/new payload fields are reported as request-contract errors, not Display name errors... '
bad_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
stamp=sys.argv[1]
print(json.dumps({
  "display_name":"Contract QA "+stamp,
  "legal_name":"Contract QA "+stamp+" LLC",
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "frontend_contract_field":"unexpected"
}))
PY
)"
code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$bad_payload" "$BASE_URL/api/v1/partners")"
test "$code" = "400"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
message=d["error"]["message"]
assert message.startswith("Invalid partner request:"), message
assert "Display name is required" not in message, message
PY
echo ok

printf 'an actually empty display name still returns the dedicated display-name validation... '
empty_payload='{"display_name":"","legal_name":"Empty QA LLC","category_id":"cat_006","lifecycle":"PROSPECT"}'
code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$empty_payload" "$BASE_URL/api/v1/partners")"
test "$code" = "400"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
assert d["error"]["message"]=="Display name is required", d
PY
echo ok

printf 'a duplicate primary domain returns a precise conflict without blaming the display name... '
domain_one="$(python3 - "$STAMP" <<'PY'
import json,sys
stamp=sys.argv[1]
print(json.dumps({
  "display_name":"Domain QA A "+stamp,
  "legal_name":"Domain QA A "+stamp+" LLC",
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "primary_domain":"domain-qa-"+stamp+".example"
}))
PY
)"
domain_two="$(python3 - "$STAMP" <<'PY'
import json,sys
stamp=sys.argv[1]
print(json.dumps({
  "display_name":"Domain QA B "+stamp,
  "legal_name":"Domain QA B "+stamp+" LLC",
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "primary_domain":"domain-qa-"+stamp+".example"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$domain_one" "$BASE_URL/api/v1/partners" >/dev/null
code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$domain_two" "$BASE_URL/api/v1/partners")"
test "$code" = "409"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
message=d["error"]["message"]
assert "Primary domain" in message, message
assert "Display name is required" not in message, message
PY
echo ok

echo 'HIMATE START-23.11.3g partner registration error-handling smoke passed'
