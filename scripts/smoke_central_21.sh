#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE="/tmp/himate-central21-owner.txt"
HEADERS="/tmp/himate-central21-headers.txt"
BODY="/tmp/himate-central21-body.json"
PAUSED="partners catalog billing payments backups evidence impact reports cms environments connector provisioning tenantfinance health"

cleanup() {
  docker compose unpause $PAUSED >/dev/null 2>&1 || true
  rm -f "$COOKIE" "$HEADERS" "$BODY"
}
trap cleanup EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

LOGIN="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl --max-time 3 -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$LOGIN" "$BASE_URL/api/v1/auth/login" >/dev/null

TEST_PARTNER_ID="$(docker compose exec -T postgres psql -U himate -d himate -At -c "SELECT id FROM partners.partners WHERE test_partner=TRUE AND lower(trim(display_name))='test partner' ORDER BY created_at DESC LIMIT 1")"
test -n "$TEST_PARTNER_ID"

printf 'CENTRAL-21 persistent LKG database coverage... '
STATE="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' <<'SQL'
SELECT
  (SELECT COUNT(*) FROM identity.central_screen_snapshots WHERE payload->>'status'='healthy'),
  (SELECT COUNT(*) FROM identity.central_screen_snapshots WHERE COALESCE(payload->>'status','')<>'healthy'),
  (SELECT COUNT(*) FROM identity.partner_workspace_snapshots WHERE payload->>'status'='healthy'),
  (SELECT COUNT(*) FROM identity.partner_workspace_snapshots WHERE COALESCE(payload->>'status','')<>'healthy'),
  (SELECT COUNT(*) FROM partners.partners);
SQL
)"
printf '%s' "$STATE" | python3 -c '
import sys
healthy_c,bad_c,healthy_t,bad_t,partners=map(int,sys.stdin.read().strip().split("|"))
assert healthy_c >= 12,(healthy_c,bad_c)
assert bad_c == 0,(healthy_c,bad_c)
assert bad_t == 0,(healthy_t,bad_t)
assert healthy_t >= partners,(healthy_t,partners)
'
echo ok

check_read() {
  path="$1"
  expected_cache="$2"
  curl --max-time 2 -fsS -D "$HEADERS" -o "$BODY" -b "$COOKIE" "$BASE_URL$path"
  grep -Eiq "^X-Himate-Cache: ($expected_cache)\r?$" "$HEADERS" || {
    echo "Unexpected read-model cache header for $path"
    cat "$HEADERS"
    exit 1
  }
  python3 - "$BODY" <<'PY'
import json,sys
with open(sys.argv[1],encoding="utf-8") as f:
    data=json.load(f)
if isinstance(data,dict):
    status=str(data.get("status","")).lower()
    meta=data.get("meta") if isinstance(data.get("meta"),dict) else {}
    meta_status=str(meta.get("status","")).lower()
    assert status not in {"partial","unavailable","warming"},data
    assert meta_status not in {"partial","unavailable","warming"},data
PY
}

printf 'CENTRAL-21 baseline materialized REST reads... '
check_read "/api/v1/partners?limit=5&offset=0" "persistent-read-model"
check_read "/api/v1/modules" "persistent-read-model"
check_read "/api/v1/billing/plans" "persistent-read-model"
check_read "/api/v1/billing/finance/overview" "persistent-read-model"
check_read "/api/v1/system-health/snapshot" "persistent-read-model"
check_read "/api/v1/cms/pages" "persistent-read-model"
check_read "/api/v1/impact/summary" "persistent-read-model"
check_read "/api/v1/reports" "persistent-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/portal-users" "persistent-tenant-read-model"
check_read "/api/v1/provisioning/jobs?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/impact/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/evidence?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
echo ok

printf 'CENTRAL-21 pause transactional/upstream services... '
docker compose pause $PAUSED >/dev/null
echo ok

printf 'CENTRAL-21 zero-fan-out reads survive dependency outage... '
# Repeat every critical read with owner services frozen. Any synchronous fan-out
# would now hit the 2s curl deadline or return a 5xx.
check_read "/api/v1/partners?limit=5&offset=0" "persistent-read-model"
check_read "/api/v1/modules" "persistent-read-model"
check_read "/api/v1/billing/plans" "persistent-read-model"
check_read "/api/v1/billing/finance/overview" "persistent-read-model"
check_read "/api/v1/system-health/snapshot" "persistent-read-model"
check_read "/api/v1/cms/pages" "persistent-read-model"
check_read "/api/v1/impact/summary" "persistent-read-model"
check_read "/api/v1/reports" "persistent-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/portal-users" "persistent-tenant-read-model"
check_read "/api/v1/provisioning/jobs?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/impact/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/evidence?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
echo ok

docker compose unpause $PAUSED >/dev/null
echo 'CENTRAL-21 zero-fan-out persistent CQRS runtime acceptance passed'
