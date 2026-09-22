#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-post223-privacy-owner.txt"
HEADERS="$TMP_ROOT/himate-post223-privacy-headers.txt"
BODY="$TMP_ROOT/himate-post223-privacy-body.json"
rm -f "$COOKIE" "$HEADERS" "$BODY"
trap 'rm -f "$COOKIE" "$HEADERS" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'privacy audit: owner login cookie flags... '
payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -D "$HEADERS" -c "$COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login" >"$BODY"
grep -qi 'set-cookie:.*HttpOnly' "$HEADERS"
grep -qi 'set-cookie:.*SameSite=Strict' "$HEADERS"
if grep -qi 'set-cookie:.*Path=/partner' "$HEADERS"; then
  echo "administrator cookie unexpectedly scoped to /partner" >&2
  exit 1
fi
echo ok

printf 'privacy audit: unauthenticated administration is denied... '
test "$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/api/v1/admin/users")" = "401"
echo ok

printf 'privacy audit: administration responses expose no credential material... '
users="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/admin/users")"
profile="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/profile")"
printf '%s\n%s' "$users" "$profile" | python3 -c '
import json,sys
raw=sys.stdin.read()
for forbidden in ("password_hash","password_changed_at","session_version","himate_session"):
    assert forbidden not in raw, forbidden
'
echo ok

printf 'privacy audit: cross-origin mutation is rejected... '
code="$(status "$COOKIE" PATCH "/api/v1/profile" -H 'Origin: https://evil.example' -H 'Content-Type: application/json' -d '{"job_title":"SHOULD_NOT_APPLY"}')"
test "$code" = "403"
echo ok

printf 'privacy audit: connector raw credential is one-time and not persisted in admin reads... '
credential="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"environment":"STAGING"}' "$BASE_URL/api/v1/connectors/ptr_000001/credential")"
token="$(printf '%s' "$credential" | python3 -c 'import json,sys; d=json.load(sys.stdin); t=d.get("token",""); assert t.startswith("hmc_crd_"); print(t)')"
metadata="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/connectors/ptr_000001/credential")"
if printf '%s' "$metadata" | grep -Fq "$token"; then
  echo "raw connector token leaked through credential metadata" >&2
  exit 1
fi
if printf '%s' "$metadata" | grep -Eq '"token_hash"|"token"[[:space:]]*:'; then
  echo "connector secret material exposed through GET metadata" >&2
  exit 1
fi
echo ok

printf 'privacy audit: connector credential cannot authenticate as HIMATE administrator... '
test "$(curl -sS -o "$BODY" -w '%{http_code}' -H "Authorization: Bearer $token" "$BASE_URL/api/v1/auth/me")" = "401"
echo ok

printf 'privacy audit: raw connector credential is redacted from central audit... '
sleep 1
audit="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/audit/events?limit=200")"
if printf '%s' "$audit" | grep -Fq "$token"; then
  echo "raw connector credential leaked into audit history" >&2
  exit 1
fi
printf '%s' "$audit" | python3 -c '
import json,sys
d=json.load(sys.stdin)
items=d.get("items",[])
assert any(x.get("resource")=="connectors" for x in items), "connector mutation missing from audit"
'
echo ok

printf 'privacy audit: retained connector records do not expose data without admin session... '
test "$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/api/v1/connectors/start22/records?partner_id=ptr_000001")" = "401"
echo ok

echo "HIMATE post-START-22.3 privacy/security audit passed"
