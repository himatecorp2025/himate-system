#!/bin/sh
set -eu

BASE="${1:-http://127.0.0.1:8080}"
TMP_HEADERS="$(mktemp)"
TMP_BODY="$(mktemp)"
TMP_PAYLOAD="$(mktemp)"
trap 'rm -f "$TMP_HEADERS" "$TMP_BODY" "$TMP_PAYLOAD"' EXIT

echo "START-24 security headers and API cache policy..."
curl -fsS -D "$TMP_HEADERS" -o /dev/null "$BASE/api/v1/live"
grep -qi '^X-Content-Type-Options: nosniff' "$TMP_HEADERS"
grep -qi '^X-Frame-Options: DENY' "$TMP_HEADERS"
grep -qi '^Content-Security-Policy:' "$TMP_HEADERS"
grep -qi '^Cache-Control: no-store' "$TMP_HEADERS"

echo "START-24 cross-site browser mutation rejection..."
status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}' \
  -X POST \
  -H 'Origin: https://evil.example' \
  -H 'Sec-Fetch-Site: cross-site' \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"Local-Development1!Password"}' \
  "$BASE/api/v1/auth/login")"
test "$status" = "403"
grep -q '"code":"CSRF"' "$TMP_BODY"

echo "START-24 forged session rejection..."
status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}' \
  -H 'Cookie: himate_session=forged.payload' \
  "$BASE/api/v1/auth/me")"
test "$status" = "401"

echo "START-24 authentication throttle and per-client isolation..."
i=0
while [ "$i" -lt 5 ]; do
  status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}' \
    -X POST \
    -H 'X-Forwarded-For: 198.51.100.240' \
    -H 'Content-Type: application/json' \
    -d '{"email":"start24-missing@example.com","password":"Wrong-Password1!"}' \
    "$BASE/api/v1/auth/login")"
  test "$status" = "401"
  i=$((i+1))
done
status="$(curl -sS -D "$TMP_HEADERS" -o "$TMP_BODY" -w '%{http_code}' \
  -X POST \
  -H 'X-Forwarded-For: 198.51.100.240' \
  -H 'Content-Type: application/json' \
  -d '{"email":"start24-missing@example.com","password":"Wrong-Password1!"}' \
  "$BASE/api/v1/auth/login")"
test "$status" = "429"
grep -qi '^Retry-After: 900' "$TMP_HEADERS"
status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}' \
  -X POST \
  -H 'X-Forwarded-For: 198.51.100.241' \
  -H 'Content-Type: application/json' \
  -d '{"email":"start24-missing@example.com","password":"Wrong-Password1!"}' \
  "$BASE/api/v1/auth/login")"
test "$status" = "401"

echo "START-24 oversized JSON request remains bounded..."
python3 - "$TMP_PAYLOAD" <<'PY'
import json
import sys
with open(sys.argv[1], "w") as handle:
    json.dump({"email":"oversized@example.com","password":"A"*1100000}, handle)
PY
status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}' \
  -X POST \
  -H 'X-Forwarded-For: 198.51.100.242' \
  -H 'Content-Type: application/json' \
  --data-binary "@$TMP_PAYLOAD" \
  "$BASE/api/v1/auth/login")"
test "$status" = "400"

echo "START-24 unsigned Stripe webhook rejection..."
status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}' \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"id":"evt_start24_unsigned","type":"payment_intent.succeeded"}' \
  "$BASE/webhooks/stripe")"
test "$status" = "400"
grep -q '"code":"INVALID_SIGNATURE"' "$TMP_BODY"

echo "START-24 post-abuse service health..."
curl -fsS "$BASE/api/v1/health" | grep -q '"status":"ok"'

echo "START-24 runtime security acceptance: PASS"
