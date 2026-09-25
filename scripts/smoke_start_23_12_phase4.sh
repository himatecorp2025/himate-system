#!/bin/sh
set -eu

BASE="${1:-http://127.0.0.1:8080}"
TMP_HEADERS="$(mktemp)"
TMP_BODY="$(mktemp)"
COOKIE="$(mktemp)"
trap 'rm -f "$TMP_HEADERS" "$TMP_BODY" "$COOKIE"' EXIT

echo "phase4 security headers..."
curl -fsS -D "$TMP_HEADERS" -o /dev/null "$BASE/api/v1/live"
grep -qi '^X-Content-Type-Options: nosniff' "$TMP_HEADERS"
grep -qi '^X-Frame-Options: DENY' "$TMP_HEADERS"
grep -qi '^Content-Security-Policy:' "$TMP_HEADERS"

echo "phase4 cross-site admin login is rejected..."
status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}'   -X POST   -H 'Origin: https://evil.example'   -H 'Sec-Fetch-Site: cross-site'   -H 'Content-Type: application/json'   -d '{"email":"admin@example.com","password":"Local-Development1!Password"}'   "$BASE/api/v1/auth/login")"
test "$status" = "403"
grep -q '"code":"CSRF"' "$TMP_BODY"

echo "phase4 cross-site Partner login is rejected..."
status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}'   -X POST   -H 'Origin: https://evil.example'   -H 'Sec-Fetch-Site: cross-site'   -H 'Content-Type: application/json'   -d '{"email":"nobody@example.com","password":"invalid"}'   "$BASE/partner/api/v1/auth/login")"
test "$status" = "403"

echo "phase4 originless API compatibility remains intact..."
status="$(curl -sS -o "$TMP_BODY" -w '%{http_code}'   -X POST -H 'Content-Type: application/json'   -d '{"email":"missing-phase4@example.com","password":"Not-The-Password1!"}'   "$BASE/api/v1/auth/login")"
test "$status" = "401"

echo "phase4 inherited admin login remains usable in Compose..."
status="$(curl -sS -c "$COOKIE" -o "$TMP_BODY" -w '%{http_code}'   -X POST -H 'Content-Type: application/json'   -d '{"email":"admin@example.com","password":"Local-Development1!Password","remember":false}'   "$BASE/api/v1/auth/login")"
test "$status" = "200"
grep -q '"email":"admin@example.com"' "$TMP_BODY"
curl -fsS -b "$COOKIE" "$BASE/api/v1/auth/me" | grep -q '"email":"admin@example.com"'

echo "START-23.12 Phase 4 runtime security smoke: PASS"
