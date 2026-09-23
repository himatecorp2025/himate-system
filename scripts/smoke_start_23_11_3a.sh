#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113a-owner.txt"
BODY="$TMP_ROOT/himate-start23113a-body.json"
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

printf 'platform secret catalog is write-only and initially readable as metadata... '
catalog="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/admin/secrets")"
printf '%s' "$catalog" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["storage"]=="AES_256_GCM_ENCRYPTED"; assert d["values_readable"] is False; assert len(d["items"])==3; assert {x["key"] for x in d["items"]}=={"stripe_secret_key","stripe_webhook_secret","render_api_key"}; assert all(x.get("value") is None for x in d["items"])'
echo ok

STAMP="$(date +%s)"
DUMMY="sk_test_start23113a_$STAMP"
payload="$(python3 - "$DUMMY" <<'PY'
import json,sys
print(json.dumps({"secret":sys.argv[1]}))
PY
)"

printf 'system owner can store an allowlisted secret without echoing its raw value... '
response="$(curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/admin/secrets/stripe_secret_key")"
printf '%s' "$response" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["key"]=="stripe_secret_key"; assert d["configured"] is True; assert d["status"]=="CONFIGURED"; assert d.get("value") is None'
if printf '%s' "$response" | grep -F "$DUMMY" >/dev/null; then
  echo 'raw secret leaked in mutation response' >&2
  exit 1
fi
echo ok

printf 'secret status remains readable while raw value remains undisclosed... '
after="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/admin/secrets")"
printf '%s' "$after" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next(v for v in d["items"] if v["key"]=="stripe_secret_key"); assert x["configured"] is True; assert x.get("value") is None'
if printf '%s' "$after" | grep -F "$DUMMY" >/dev/null; then
  echo 'raw secret leaked in metadata response' >&2
  exit 1
fi
echo ok

printf 'arbitrary secret names are rejected... '
code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/admin/secrets/arbitrary_untrusted_key")"
test "$code" = "404"
grep -q 'SECRET_NOT_SUPPORTED' "$BODY"
echo ok

printf 'system owner can remove the stored secret and return to configuration-required state... '
curl -fsS -b "$OWNER_COOKIE" -X DELETE "$BASE_URL/api/v1/admin/secrets/stripe_secret_key" >/dev/null
final="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/admin/secrets")"
printf '%s' "$final" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next(v for v in d["items"] if v["key"]=="stripe_secret_key"); assert x["configured"] is False; assert x["status"]=="MISSING"; assert x.get("value") is None'
echo ok

echo 'HIMATE START-23.11.3a Platform Secrets smoke passed'
