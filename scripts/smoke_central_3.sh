#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE_JAR="/tmp/himate-central3-cookies.txt"
rm -f "$COOKIE_JAR"
trap 'rm -f "$COOKIE_JAR"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

printf 'Central-3 owner login... '
LOGIN="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json' -d "$LOGIN" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'Central-3 Golden/Test Partner remains visible in core portfolio... '
TEST_PARTNERS="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners?limit=200&offset=0&core_only=true&q=Golden%20Test%20Partner")"
printf '%s' "$TEST_PARTNERS" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["count"] >= 1,d
assert any(x.get("test_partner") is True for x in d["items"]),d
assert all(x.get("lifecycle")!="ARCHIVED" for x in d["items"]),d
'
echo ok

printf 'Central-3 reference KPI filter is authoritative... '
REFERENCE="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners?limit=200&offset=0&core_only=true&reference=true")"
printf '%s' "$REFERENCE" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["count"] >= 1,d
assert all(x.get("reference_partner") is True for x in d["items"]),d
'
REFERENCE_STATS="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners?stats_only=true&core_only=true&reference=true")"
printf '%s' "$REFERENCE_STATS" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["total"] >= 1,d
assert d["reference_count"] == d["total"],d
'
echo ok

printf 'Central-3 lifecycle KPI filter returns only requested state... '
LIVE="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/partners?limit=200&offset=0&core_only=true&lifecycle=LIVE")"
printf '%s' "$LIVE" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["count"] >= 1,d
assert all(x.get("lifecycle")=="LIVE" for x in d["items"]),d
'
echo ok

echo "HIMATE Central-3 Partners runtime smoke passed"
