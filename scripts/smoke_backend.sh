#!/usr/bin/env sh
set -eu
BASE_URL="${1:-http://localhost:8080}"
curl -fsS "$BASE_URL/api/v1/health"
printf '\n'
