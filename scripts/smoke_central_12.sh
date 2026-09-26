#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-central12-owner.txt"
HDR="$TMP_ROOT/himate-central12-headers.txt"
BODY="$TMP_ROOT/himate-central12-body.json"
rm -f "$COOKIE" "$HDR" "$BODY"
trap 'rm -f "$COOKIE" "$HDR" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
LOGIN="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$LOGIN" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'CENTRAL-12 health gate... '
curl -fsS "$BASE_URL/api/v1/health" | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"]=="ok"'
echo ok

printf 'wait for Partners materialized hot snapshot... '
i=0
while [ "$i" -lt 30 ]; do
  curl -fsS -D "$HDR" -o "$BODY" -b "$COOKIE" "$BASE_URL/api/v1/central/partners?limit=24&offset=0"
  if grep -qi '^X-Himate-Cache: hot-snapshot' "$HDR"; then
    break
  fi
  i=$((i+1))
  sleep 1
done
grep -qi '^X-Himate-Cache: hot-snapshot' "$HDR"
python3 - "$BODY" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
assert isinstance(d.get("items"), list), d
assert isinstance(d.get("pagination"), dict), d
assert isinstance(d.get("kpis"), dict), d
assert d.get("meta",{}).get("delivery") == "MATERIALIZED_HOT_SNAPSHOT", d
PY
echo ok

printf 'Partners hot read stays inside 800ms budget... '
PARTNERS_TIME="$(curl -fsS -o "$BODY" -w '%{time_total}' -b "$COOKIE" "$BASE_URL/api/v1/central/partners?limit=24&offset=0")"
python3 - "$PARTNERS_TIME" <<'PY'
import sys
value=float(sys.argv[1])
assert value < 0.8, value
PY
echo ok

printf 'wait for Finance materialized snapshot... '
i=0
while [ "$i" -lt 30 ]; do
  curl -fsS -o "$BODY" -b "$COOKIE" "$BASE_URL/api/v1/central/finance"
  READY="$(python3 - "$BODY" <<'PY'
import json,sys
print("yes" if json.load(open(sys.argv[1])).get("ready") is True else "no")
PY
)"
  [ "$READY" = "yes" ] && break
  i=$((i+1))
  sleep 1
done
[ "$READY" = "yes" ]
echo ok

printf 'Finance hot read stays inside 800ms budget... '
FINANCE_TIME="$(curl -fsS -o "$BODY" -w '%{time_total}' -b "$COOKIE" "$BASE_URL/api/v1/central/finance")"
python3 - "$FINANCE_TIME" <<'PY'
import sys
value=float(sys.argv[1])
assert value < 0.8, value
PY
echo ok

printf 'Package Analytics is authoritative runtime data... '
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/packages/analytics" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d["packages"],list),d; assert isinstance(d["partners"],list),d; assert d["source"]=="BILLING_SUBSCRIPTIONS_CATALOG_USAGE_PORTAL_ACTIVITY",d'
echo ok

printf 'empty Partners export is explicitly reported... '
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/export.pdf?q=__CENTRAL12_NO_MATCH__&availability=1" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["has_data"] is False,d; assert d["count"]==0,d'
echo ok

printf 'empty Finance export is explicitly reported... '
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/finance/export.pdf?plan_key=__CENTRAL12_NO_MATCH__&availability=1" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["has_data"] is False,d; assert d["count"]==0,d'
echo ok

printf 'empty Impact export is explicitly reported... '
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/impact/export.pdf?partner_id=__CENTRAL12_NO_MATCH__&availability=1" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["has_data"] is False,d; assert d["count"]==0,d'
echo ok

printf 'Package export exposes availability instead of an empty PDF contract... '
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/packages/export.pdf?availability=1" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d["has_data"],bool),d; assert isinstance(d["count"],int),d'
echo ok

echo 'CENTRAL-12 performance, analytics and PDF-empty-state runtime acceptance passed'
