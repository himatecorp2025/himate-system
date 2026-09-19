#!/bin/sh
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:10000}"
COOKIE_FILE="${COOKIE_FILE:-/tmp/himate-smoke-cookie.txt}"
: "${HIMATE_BOOTSTRAP_ADMIN_EMAIL:?Set HIMATE_BOOTSTRAP_ADMIN_EMAIL}"
: "${HIMATE_BOOTSTRAP_ADMIN_PASSWORD:?Set HIMATE_BOOTSTRAP_ADMIN_PASSWORD}"

curl -fsS "$BASE_URL/api/v1/health"
printf '\n'

curl -fsS -c "$COOKIE_FILE" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$HIMATE_BOOTSTRAP_ADMIN_EMAIL\",\"password\":\"$HIMATE_BOOTSTRAP_ADMIN_PASSWORD\"}" \
  "$BASE_URL/api/v1/auth/login"
printf '\n'

curl -fsS -b "$COOKIE_FILE" "$BASE_URL/api/v1/auth/me"
printf '\n'

curl -fsS -b "$COOKIE_FILE" "$BASE_URL/api/v1/dashboard/summary"
printf '\n'
