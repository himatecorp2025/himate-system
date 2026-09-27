#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE="/tmp/himate-central21-owner.txt"
PARTNER_COOKIE="/tmp/himate-central21-partner.txt"
HEADERS="/tmp/himate-central21-headers.txt"
BODY="/tmp/himate-central21-body.json"
PAUSED="partners catalog billing payments backups evidence impact reports cms environments connector provisioning tenantfinance health"

cleanup() {
  docker compose unpause $PAUSED >/dev/null 2>&1 || true
  rm -f "$COOKIE" "$PARTNER_COOKIE" "$HEADERS" "$BODY"
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
MODULE_KEY="$(docker compose exec -T postgres psql -U himate -d himate -At -c "SELECT module_key FROM catalog.modules ORDER BY module_key LIMIT 1")"
test -n "$MODULE_KEY"

printf 'CENTRAL-21 create isolated Golden Test Partner portal owner... '
PARTNER_PASSWORD="$(python3 -c 'import secrets; print("Cq21!"+secrets.token_urlsafe(24))')"
PARTNER_EMAIL="central21-portal-owner@example.com"
PARTNER_PAYLOAD="$(python3 - "$PARTNER_PASSWORD" "$PARTNER_EMAIL" <<'PY'
import json,sys
print(json.dumps({
  "name":"CENTRAL-21 Portal Owner",
  "email":sys.argv[2],
  "password":sys.argv[1],
  "role":"owner"
},separators=(",",":")))
PY
)"
curl --max-time 5 -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$PARTNER_PAYLOAD"   "$BASE_URL/api/v1/partners/$TEST_PARTNER_ID/portal-users" >/dev/null
PARTNER_LOGIN="$(python3 - "$PARTNER_EMAIL" "$PARTNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False},separators=(",",":")))
PY
)"
curl --max-time 3 -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$PARTNER_LOGIN"   "$BASE_URL/partner/api/v1/auth/login" >/dev/null
curl --max-time 2 -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/auth/me" |   python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]' "$TEST_PARTNER_ID"
echo ok

printf 'CENTRAL-21 persistent LKG database coverage... '
STATE="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' <<'SQL'
SELECT
  (SELECT COUNT(*) FROM identity.central_screen_snapshots WHERE payload->>'status'='healthy'),
  (SELECT COUNT(*) FROM identity.central_screen_snapshots WHERE COALESCE(payload->>'status','')<>'healthy'),
  (SELECT COUNT(*) FROM identity.partner_workspace_snapshots WHERE payload->>'status'='healthy'),
  (SELECT COUNT(*) FROM identity.partner_workspace_snapshots WHERE COALESCE(payload->>'status','')<>'healthy'),
  (SELECT COUNT(*) FROM partners.partners),
  (SELECT COUNT(*) FROM identity.partner_workspace_snapshots
   WHERE NOT (
     payload ? 'partner_contacts'
     AND payload ? 'partner_domains_deployments'
     AND payload ? 'partner_audit_events'
     AND payload ? 'partner_permissions'
     AND payload ? 'module_commercial_history'
     AND payload ? 'start22_summary'
     AND payload ? 'start22_retention'
   ));
SQL
)"
printf '%s' "$STATE" | python3 -c '
import sys
healthy_c,bad_c,healthy_t,bad_t,partners,incomplete_t=map(int,sys.stdin.read().strip().split("|"))
assert healthy_c >= 12,(healthy_c,bad_c)
assert bad_c == 0,(healthy_c,bad_c)
assert bad_t == 0,(healthy_t,bad_t)
assert healthy_t >= partners,(healthy_t,partners)
assert incomplete_t == 0,incomplete_t
'
echo ok

check_read() {
  path="$1"
  expected_cache="$2"
  enforce_slo="${3:-false}"
  TTFB="$(curl --max-time 2 -fsS -w '%{time_starttransfer}' -D "$HEADERS" -o "$BODY" -b "$COOKIE" "$BASE_URL$path")"
  grep -Eiq "^X-Himate-Cache: ($expected_cache)\r?$" "$HEADERS" || {
    echo "Unexpected read-model cache header for $path"
    cat "$HEADERS"
    exit 1
  }
  python3 - "$BODY" "$TTFB" "$enforce_slo" <<'PY'
import json,sys
with open(sys.argv[1],encoding="utf-8") as f:
    data=json.load(f)
if isinstance(data,dict):
    status=str(data.get("status","")).lower()
    meta=data.get("meta") if isinstance(data.get("meta"),dict) else {}
    meta_status=str(meta.get("status","")).lower()
    assert status not in {"partial","unavailable","warming"},data
    assert meta_status not in {"partial","unavailable","warming"},data
if sys.argv[3].lower()=="true":
    ttfb=float(sys.argv[2])
    assert ttfb <= 0.020, f"materialized GET TTFB {ttfb*1000:.3f}ms exceeds 20ms SLO"
PY
}

check_partner_read() {
  path="$1"
  enforce_slo="${2:-false}"
  TTFB="$(curl --max-time 2 -fsS -w '%{time_starttransfer}' -D "$HEADERS" -o "$BODY" -b "$PARTNER_COOKIE" "$BASE_URL$path")"
  grep -Eiq '^X-Himate-Cache: persistent-tenant-read-model\r?
check_read "/api/v1/partners?limit=5&offset=0" "persistent-read-model" "true"
check_read "/api/v1/modules" "persistent-read-model" "true"
check_read "/api/v1/modules/$MODULE_KEY/relationships" "persistent-read-model" "true"
check_read "/api/v1/modules/$MODULE_KEY/impact-metrics" "persistent-read-model" "true"
check_read "/api/v1/modules/$MODULE_KEY/usage" "persistent-read-model" "true"
check_read "/api/v1/connectors/start22/mapping" "persistent-read-model" "true"
check_read "/api/v1/billing/plans" "persistent-read-model" "true"
check_read "/api/v1/billing/finance/overview" "persistent-read-model" "true"
check_read "/api/v1/system-health/snapshot" "persistent-read-model" "true"
check_read "/api/v1/backups/restore-tests?limit=1" "persistent-read-model" "true"
check_read "/api/v1/backups/restores" "persistent-read-model" "true"
check_read "/api/v1/cms/pages" "persistent-read-model" "true"
check_read "/api/v1/impact/summary" "persistent-read-model" "true"
check_read "/api/v1/reports" "persistent-read-model" "true"
check_read "/api/v1/partners/$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules" "persistent-tenant-read-model" "true"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules/$MODULE_KEY/commercial-history" "persistent-tenant-read-model" "true"
check_read "/api/v1/connectors/start22/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/connectors/start22/retention?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/partners/$TEST_PARTNER_ID/portal-users" "persistent-tenant-read-model" "true"
check_read "/api/v1/provisioning/jobs?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/impact/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/evidence?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"

# Explicit Control Plane screens: same persistent CQRS model, historical headers preserved.
check_read "/api/v1/central/partners?limit=24&offset=0" "hot-snapshot" "true"
check_read "/api/v1/central/modules" "hot-snapshot" "true"
check_read "/api/v1/central/packages" "hot-snapshot" "true"
check_read "/api/v1/central/finance?invoice_status=ALL&revenue_period=MONTHLY&revenue_plan=ALL" "hot-snapshot" "true"
check_read "/api/v1/central/impact" "hot-snapshot" "true"
check_read "/api/v1/central/website" "persistent-read-model" "true"
check_read "/api/v1/central/administration?limit=60&offset=0" "hot-snapshot" "true"
check_read "/api/v1/central/system" "hot-snapshot" "true"
check_read "/api/v1/central/connections" "persistent-read-model" "true"
echo ok

printf 'CENTRAL-21 Partner Portal persistent read coverage... '
check_partner_read "/partner/api/v1/dashboard" "true"
check_partner_read "/partner/api/v1/company" "true"
check_partner_read "/partner/api/v1/modules" "true"
check_partner_read "/partner/api/v1/billing/summary" "true"
check_partner_read "/partner/api/v1/billing/subscriptions" "true"
check_partner_read "/partner/api/v1/billing/invoices" "true"
check_partner_read "/partner/api/v1/impact/summary" "true"
check_partner_read "/partner/api/v1/users" "true"
check_partner_read "/partner/api/v1/audit" "true"
check_partner_read "/partner/api/v1/permissions" "true"
check_partner_read "/partner/api/v1/design" "true"
echo ok

printf 'CENTRAL-21 pause transactional/upstream services... '
docker compose pause $PAUSED >/dev/null
echo ok

printf 'CENTRAL-21 zero-fan-out reads survive dependency outage... '
# Repeat every critical read with owner services frozen. Any synchronous fan-out
# would now hit the 2s curl deadline or return a 5xx.
check_read "/api/v1/partners?limit=5&offset=0" "persistent-read-model"
check_read "/api/v1/modules" "persistent-read-model"
check_read "/api/v1/modules/$MODULE_KEY/relationships" "persistent-read-model"
check_read "/api/v1/modules/$MODULE_KEY/impact-metrics" "persistent-read-model"
check_read "/api/v1/modules/$MODULE_KEY/usage" "persistent-read-model"
check_read "/api/v1/connectors/start22/mapping" "persistent-read-model"
check_read "/api/v1/billing/plans" "persistent-read-model"
check_read "/api/v1/billing/finance/overview" "persistent-read-model"
check_read "/api/v1/system-health/snapshot" "persistent-read-model"
check_read "/api/v1/backups/restore-tests?limit=1" "persistent-read-model"
check_read "/api/v1/backups/restores" "persistent-read-model"
check_read "/api/v1/cms/pages" "persistent-read-model"
check_read "/api/v1/impact/summary" "persistent-read-model"
check_read "/api/v1/reports" "persistent-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules/$MODULE_KEY/commercial-history" "persistent-tenant-read-model"
check_read "/api/v1/connectors/start22/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/connectors/start22/retention?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/portal-users" "persistent-tenant-read-model"
check_read "/api/v1/provisioning/jobs?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/impact/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/evidence?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"

check_read "/api/v1/central/partners?limit=24&offset=0" "hot-snapshot"
check_read "/api/v1/central/modules" "hot-snapshot"
check_read "/api/v1/central/packages" "hot-snapshot"
check_read "/api/v1/central/finance?invoice_status=ALL&revenue_period=MONTHLY&revenue_plan=ALL" "hot-snapshot"
check_read "/api/v1/central/impact" "hot-snapshot"
check_read "/api/v1/central/website" "persistent-read-model"
check_read "/api/v1/central/administration?limit=60&offset=0" "hot-snapshot"
check_read "/api/v1/central/system" "hot-snapshot"
check_read "/api/v1/central/connections" "persistent-read-model"

check_partner_read "/partner/api/v1/dashboard"
check_partner_read "/partner/api/v1/company"
check_partner_read "/partner/api/v1/modules"
check_partner_read "/partner/api/v1/billing/summary"
check_partner_read "/partner/api/v1/billing/subscriptions"
check_partner_read "/partner/api/v1/billing/invoices"
check_partner_read "/partner/api/v1/impact/summary"
check_partner_read "/partner/api/v1/users"
check_partner_read "/partner/api/v1/audit"
check_partner_read "/partner/api/v1/permissions"
check_partner_read "/partner/api/v1/design"
echo ok

docker compose unpause $PAUSED >/dev/null
echo 'CENTRAL-21 zero-fan-out persistent CQRS runtime acceptance passed'
 "$HEADERS" || {
    echo "Unexpected Partner Portal read-model cache header for $path"
    cat "$HEADERS"
    exit 1
  }
  python3 - "$BODY" "$TTFB" "$enforce_slo" <<'PY'
import json,sys
with open(sys.argv[1],encoding="utf-8") as f:
    data=json.load(f)
if isinstance(data,dict):
    status=str(data.get("status","")).lower()
    meta=data.get("meta") if isinstance(data.get("meta"),dict) else {}
    meta_status=str(meta.get("status","")).lower()
    assert status not in {"partial","unavailable","warming"},data
    assert meta_status not in {"partial","unavailable","warming"},data
if sys.argv[3].lower()=="true":
    ttfb=float(sys.argv[2])
    assert ttfb <= 0.020, f"partner materialized GET TTFB {ttfb*1000:.3f}ms exceeds 20ms SLO"
PY
}

printf 'CENTRAL-21 baseline materialized REST reads and <=20ms local SLO... '
check_read "/api/v1/partners?limit=5&offset=0" "persistent-read-model" "true"
check_read "/api/v1/modules" "persistent-read-model" "true"
check_read "/api/v1/modules/$MODULE_KEY/relationships" "persistent-read-model" "true"
check_read "/api/v1/modules/$MODULE_KEY/impact-metrics" "persistent-read-model" "true"
check_read "/api/v1/modules/$MODULE_KEY/usage" "persistent-read-model" "true"
check_read "/api/v1/connectors/start22/mapping" "persistent-read-model" "true"
check_read "/api/v1/billing/plans" "persistent-read-model" "true"
check_read "/api/v1/billing/finance/overview" "persistent-read-model" "true"
check_read "/api/v1/system-health/snapshot" "persistent-read-model" "true"
check_read "/api/v1/backups/restore-tests?limit=1" "persistent-read-model" "true"
check_read "/api/v1/backups/restores" "persistent-read-model" "true"
check_read "/api/v1/cms/pages" "persistent-read-model" "true"
check_read "/api/v1/impact/summary" "persistent-read-model" "true"
check_read "/api/v1/reports" "persistent-read-model" "true"
check_read "/api/v1/partners/$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules" "persistent-tenant-read-model" "true"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules/$MODULE_KEY/commercial-history" "persistent-tenant-read-model" "true"
check_read "/api/v1/connectors/start22/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/connectors/start22/retention?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/partners/$TEST_PARTNER_ID/portal-users" "persistent-tenant-read-model" "true"
check_read "/api/v1/provisioning/jobs?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/impact/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
check_read "/api/v1/evidence?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model" "true"
echo ok

printf 'CENTRAL-21 pause transactional/upstream services... '
docker compose pause $PAUSED >/dev/null
echo ok

printf 'CENTRAL-21 zero-fan-out reads survive dependency outage... '
# Repeat every critical read with owner services frozen. Any synchronous fan-out
# would now hit the 2s curl deadline or return a 5xx.
check_read "/api/v1/partners?limit=5&offset=0" "persistent-read-model"
check_read "/api/v1/modules" "persistent-read-model"
check_read "/api/v1/modules/$MODULE_KEY/relationships" "persistent-read-model"
check_read "/api/v1/modules/$MODULE_KEY/impact-metrics" "persistent-read-model"
check_read "/api/v1/modules/$MODULE_KEY/usage" "persistent-read-model"
check_read "/api/v1/connectors/start22/mapping" "persistent-read-model"
check_read "/api/v1/billing/plans" "persistent-read-model"
check_read "/api/v1/billing/finance/overview" "persistent-read-model"
check_read "/api/v1/system-health/snapshot" "persistent-read-model"
check_read "/api/v1/backups/restore-tests?limit=1" "persistent-read-model"
check_read "/api/v1/backups/restores" "persistent-read-model"
check_read "/api/v1/cms/pages" "persistent-read-model"
check_read "/api/v1/impact/summary" "persistent-read-model"
check_read "/api/v1/reports" "persistent-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/modules/$MODULE_KEY/commercial-history" "persistent-tenant-read-model"
check_read "/api/v1/connectors/start22/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/connectors/start22/retention?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/partners/$TEST_PARTNER_ID/portal-users" "persistent-tenant-read-model"
check_read "/api/v1/provisioning/jobs?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/impact/summary?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
check_read "/api/v1/evidence?partner_id=$TEST_PARTNER_ID" "persistent-tenant-read-model"
echo ok

docker compose unpause $PAUSED >/dev/null
echo 'CENTRAL-21 zero-fan-out persistent CQRS runtime acceptance passed'
