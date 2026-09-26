#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE_JAR="/tmp/himate-central2-cookies.txt"
rm -f "$COOKIE_JAR"
trap 'rm -f "$COOKIE_JAR"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
YEAR="$(date -u +%Y)"

printf 'Central-2 owner login... '
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'Central-2 dashboard monthly/weekly Impact contract... '
dashboard="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
printf '%s' "$dashboard" | python3 -c '
import json,sys
d=json.load(sys.stdin)
impact=d["impact"]
monthly=impact["trend"]
weekly=impact["weekly_trend"]
assert impact["source"]=="IMPACT_METRIC_VALUES", impact
if impact.get("has_data") is False:
    assert monthly==[], monthly
    assert weekly==[], weekly
else:
    assert len(monthly)==12, len(monthly)
    assert len(weekly)==4, len(weekly)
    assert all("label" in x and "value" in x for x in monthly), monthly
    assert all("label" in x and "value" in x and "week" in x and "week_start" in x and "iso_year" in x for x in weekly), weekly
'
echo ok

printf 'Central-2 Recent Activity uses authoritative append-only audit feed... '
printf '%s' "$dashboard" | python3 -c '
import json,sys
d=json.load(sys.stdin)
activity=d["activity"]
assert activity["source"]=="IDENTITY_APPEND_ONLY_AUDIT", activity
assert isinstance(activity["items"],list), activity
assert activity["count"]==len(activity["items"]), activity
assert activity["count"]>=1, activity
assert all(x.get("outcome")=="SUCCESS" for x in activity["items"]), activity
'
echo ok

echo "HIMATE Central-2 Dashboard runtime smoke passed"
