#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE="/tmp/himate-central18-owner.txt"
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

printf 'CENTRAL-18 canonical package baseline... '
PLANS="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/plans")"
printf '%s' "$PLANS" | python3 -c '
import json,sys
d=json.load(sys.stdin)
rows={x["plan_key"]:x for x in d["items"]}
assert float(rows["STARTER"]["monthly_price"])==990,rows["STARTER"]
assert int(rows["STARTER"]["module_limit"])==10,rows["STARTER"]
assert float(rows["BUSINESS"]["monthly_price"])==1490,rows["BUSINESS"]
assert int(rows["BUSINESS"]["module_limit"])==20,rows["BUSINESS"]
assert float(rows["FLEX"]["monthly_price"])==2490,rows["FLEX"]
assert int(rows["FLEX"]["module_limit"])==0,rows["FLEX"]
assert rows["FLEX"]["selection_mode"]=="UNLIMITED",rows["FLEX"]
'
echo ok

printf 'CENTRAL-18 filtered Partners read model is bounded and paginated... '
FILTERED="$(curl --max-time 5 -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/partners?lifecycle=READY_TO_PROVISION&limit=24&offset=0")"
printf '%s' "$FILTERED" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert isinstance(d.get("items"),list),d
p=d.get("pagination",{})
assert p.get("limit")==24,p
assert p.get("offset")==0,p
assert isinstance(p.get("total"),int),p
assert "kpis" in d and "meta" in d,d
assert all(str(x.get("lifecycle","")).upper()=="READY_TO_PROVISION" for x in d["items"]),d["items"]
'
echo ok

printf 'CENTRAL-18 dashboard exposes the complete clickable US state domain... '
YEAR="$(date -u +%Y)"
DASHBOARD="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
printf '%s' "$DASHBOARD" | python3 -c '
import json,sys
d=json.load(sys.stdin)
g=d["geography"]
rows=g["states"]
names={x["state"] for x in rows}
assert len(rows)>=51,len(rows)
for state in ("Alabama","Alaska","Hawaii","New York","Texas","District of Columbia"):
    assert state in names,(state,names)
assert all(int(x.get("count",0))>=0 for x in rows),rows
assert int(g["active_states"])==sum(1 for x in rows if int(x.get("count",0))>0),g
'
echo ok

printf 'CENTRAL-18 system health is populated and backup creation is separately authorized... '
SYSTEM="$(curl --max-time 8 -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/system")"
printf '%s' "$SYSTEM" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d.get("ready") is True,d
health=d["health"]
assert health.get("status") not in ("",None),health
assert len(health.get("services",[]))>1,health
access=d["access"]
assert access.get("backups") is True,access
assert access.get("backups_write") is True,access
assert access.get("backups_approve") is True,access
'
echo ok

printf 'CENTRAL-18 Administration hot read model remains refresh-safe... '
ADMIN="$(curl --max-time 6 -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/administration?limit=200&offset=0")"
printf '%s' "$ADMIN" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert isinstance(d.get("items"),list),d
assert "company" in d and "kpis" in d and "meta" in d,d
'
echo ok

echo 'CENTRAL-18 data/runtime/module lifecycle runtime acceptance passed'
