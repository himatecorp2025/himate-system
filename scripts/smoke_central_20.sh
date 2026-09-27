#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE="/tmp/himate-central20-owner.txt"
rm -f "$COOKIE"
trap 'rm -f "$COOKIE"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

LOGIN="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$LOGIN" "$BASE_URL/api/v1/auth/login" >/dev/null

check_screen() {
  name="$1"
  path="$2"
  kind="$3"
  first="$(curl --max-time 3 -fsS -b "$COOKIE" "$BASE_URL$path")"
  second="$(curl --max-time 3 -fsS -b "$COOKIE" "$BASE_URL$path")"
  printf '%s\n%s' "$first" "$second" | python3 -c '
import json,sys
kind=sys.argv[1]
docs=[json.loads(x) for x in sys.stdin.read().splitlines() if x.strip()]
assert len(docs)==2,docs
for d in docs:
    meta=d.get("meta") or {}
    assert str(meta.get("status","")).lower()!="warming",d
    if kind in ("modules","packages","finance"):
        assert d.get("ready") is True,d
    if kind=="partners":
        assert isinstance(d.get("items"),list) and len(d["items"])>0,d
    elif kind=="modules":
        assert len((d.get("registry") or {}).get("modules",[]))>0,d
    elif kind=="packages":
        assert len(d.get("plans",[]))>=3,d
    elif kind=="finance":
        assert "kpis" in d and "invoices" in d and "partners" in d,d
' "$kind"
  echo "$name ok"
}

printf 'CENTRAL-20 cold/direct refresh read models... '
check_screen "partners" "/api/v1/central/partners?limit=24&offset=0" "partners"
check_screen "modules" "/api/v1/central/modules" "modules"
check_screen "packages" "/api/v1/central/packages" "packages"
check_screen "finance" "/api/v1/central/finance?invoice_status=ALL&revenue_period=MONTHLY&revenue_plan=ALL" "finance"

printf 'CENTRAL-20 Golden Test Partner has six completed months and active subscription... '
TEST_PARTNER_ID="$(docker compose exec -T postgres psql -U himate -d himate -At -c "SELECT id FROM partners.partners WHERE test_partner=TRUE AND lower(trim(display_name))='test partner' ORDER BY created_at DESC LIMIT 1")"
test -n "$TEST_PARTNER_ID"

i=0
STATE=""
while [ "$i" -lt 20 ]; do
  STATE="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' -v partner_id="$TEST_PARTNER_ID" <<'SQL'
SELECT
  (SELECT COUNT(DISTINCT date_trunc('month',service_period_start)) FROM billing.invoices WHERE partner_id=:'partner_id' AND source='TEST_FIXTURE'),
  (SELECT COUNT(*) FROM impact.metric_values WHERE partner_id=:'partner_id' AND provenance='TEST_FIXTURE'),
  (SELECT COUNT(*) FROM evidence.items WHERE partner_id=:'partner_id' AND description LIKE 'HIMATE_GOLDEN_TEST_FIXTURE%'),
  (SELECT COUNT(*) FROM billing.partner_plan_subscriptions WHERE partner_id=:'partner_id' AND status='ACTIVE' AND plan_key='CUSTOM' AND billing_frequency='MONTHLY');
SQL
)"
  if [ "$STATE" = "6|18|6|1" ]; then
    break
  fi
  i=$((i+1))
  sleep 1
done
test "$STATE" = "6|18|6|1"
echo ok

printf 'CENTRAL-20 Test Partner workspace direct route returns real history... '
WORKSPACE="$(curl --max-time 3 -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/partners/$TEST_PARTNER_ID")"
printf '%s' "$WORKSPACE" | python3 -c '
import json,sys
d=json.load(sys.stdin)
p=d.get("partner") or {}
assert p.get("id")==sys.argv[1],p
assert str(p.get("display_name","")).lower()=="test partner",p
assert len(d.get("invoices",[]))>=6,d.get("invoices")
assert len(d.get("impact_summary",[]))>0,d.get("impact_summary")
' "$TEST_PARTNER_ID"
echo ok

printf 'CENTRAL-20 module activation persists and becomes package-eligible immediately... '
CATALOG="$(curl --max-time 3 -fsS -b "$COOKIE" "$BASE_URL/api/v1/modules")"
MODULE_KEY="$(printf '%s' "$CATALOG" | python3 -c 'import json,sys; d=json.load(sys.stdin); rows=d.get("items",[]); assert rows; print(rows[0]["key"])')"
PATCH_PAYLOAD='{"availability":"ACTIVE","implementation_state":"READY","publication_status":"PUBLISHED"}'
curl --max-time 3 -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$PATCH_PAYLOAD" "$BASE_URL/api/v1/modules/$MODULE_KEY" >/dev/null

ACTIVE="$(curl --max-time 3 -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/modules?registry_preset=ACTIVE")"
printf '%s' "$ACTIVE" | python3 -c '
import json,sys
d=json.load(sys.stdin); key=sys.argv[1]
rows=(d.get("registry") or {}).get("modules",[])
m=next((x for x in rows if x.get("key")==key),None)
assert m,m
assert m.get("availability")=="ACTIVE",m
assert m.get("publication_status")=="PUBLISHED",m
assert m.get("implementation_state")=="READY",m
' "$MODULE_KEY"

SUPPLEMENTARY="$(curl --max-time 3 -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/packages/supplementary")"
printf '%s' "$SUPPLEMENTARY" | python3 -c '
import json,sys
d=json.load(sys.stdin); key=sys.argv[1]
assert d.get("ready") is True,d
keys={x.get("key") for x in d.get("modules",[])}
assert key in keys,(key,keys)
' "$MODULE_KEY"
echo ok

echo 'CENTRAL-20 eight-point runtime acceptance passed'
