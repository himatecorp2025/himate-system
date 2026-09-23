#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113b-owner.txt"
rm -f "$OWNER_COOKIE"
trap 'rm -f "$OWNER_COOKIE"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'persistent manual QA partner exists and remains LIVE... '
partner="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/ptr_himate_test_001")"
printf '%s' "$partner" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["id"]=="ptr_himate_test_001"; assert d["display_name"]=="HIMATE TEST PARTNER"; assert d["lifecycle"]=="LIVE"; assert "Persistent manual QA fixture" in d.get("notes","")'
echo ok

printf 'persistent Partner Portal owner identity is attached... '
users="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/ptr_himate_test_001/portal-users")"
printf '%s' "$users" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=d["items"]; u=next(x for x in xs if x["id"]=="pusr_himate_test_001"); assert u["partner_id"]=="ptr_himate_test_001"; assert u["email"]=="test.partner@himate.test"; assert u["role"]=="owner"; assert u["active"] is True'
echo ok

echo 'HIMATE START-23.11.3b persistent test partner smoke passed'
