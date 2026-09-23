#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113c-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-start23113c-partner.txt"
BODY="$TMP_ROOT/himate-start23113c-body.json"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

login_owner() {
  payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login" >/dev/null
}

wait_live() {
  n=0
  until curl -fsS "$BASE_URL/api/v1/live" >/dev/null 2>&1; do
    n=$((n+1))
    if [ "$n" -ge 60 ]; then
      echo "gateway did not become live after restart" >&2
      return 1
    fi
    sleep 1
  done
}

printf 'simulate an existing production DB where fixture migrations are already behind the registry... '
docker compose exec -T postgres sh -lc 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<SQL
DELETE FROM identity.partner_users WHERE partner_id = '"'"'ptr_himate_test_001'"'"';
DELETE FROM partners.partners WHERE id = '"'"'ptr_himate_test_001'"'"';
SQL' >/dev/null
echo ok

printf 'restart only the owning services so runtime idempotent seeds must repair the fixture... '
docker compose restart partners gateway >/dev/null
wait_live
echo ok

login_owner

printf 'partner service is reachable through the gateway after restart... '
health="$(curl -fsS "$BASE_URL/api/v1/health")"
printf '%s' "$health" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["services"]["partners"]=="ok", d'
echo ok

printf 'persistent QA partner is recreated outside migration-version ordering... '
partner="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/ptr_himate_test_001")"
printf '%s' "$partner" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["id"]=="ptr_himate_test_001"; assert d["lifecycle"]=="LIVE"; assert d["display_name"]=="HIMATE TEST PARTNER"'
echo ok

printf 'persistent Partner Portal identity is recreated outside migration-version ordering... '
users="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/ptr_himate_test_001/portal-users")"
printf '%s' "$users" | python3 -c 'import json,sys; d=json.load(sys.stdin); u=next(x for x in d["items"] if x["id"]=="pusr_himate_test_001"); assert u["email"]=="test.partner@himate.test"; assert u["active"] is True; assert u["role"]=="owner"'
echo ok

STAMP="$(date +%s)"
CHECK_EMAIL="runtime.partner.check.$STAMP@himate.test"
CHECK_PASSWORD="RuntimeCheck!2345Aa"
create_payload="$(python3 - "$CHECK_EMAIL" "$CHECK_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Runtime Partner Check","email":sys.argv[1],"password":sys.argv[2],"role":"viewer"}))
PY
)"

printf 'create a disposable portal identity on the persistent QA partner... '
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$create_payload" "$BASE_URL/api/v1/partners/ptr_himate_test_001/portal-users" >/dev/null
echo ok

printf 'Partner Portal login crosses the real partner-registry readiness gate successfully... '
partner_login="$(python3 - "$CHECK_EMAIL" "$CHECK_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_login" "$BASE_URL/partner/api/v1/auth/login" >"$BODY"
printf '%s' "$(cat "$BODY")" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]=="ptr_himate_test_001"; assert d["email"].startswith("runtime.partner.check.")'
curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/auth/me" >/dev/null
echo ok

docker compose exec -T postgres sh -lc "psql -v ON_ERROR_STOP=1 -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -c \"DELETE FROM identity.partner_users WHERE email='$CHECK_EMAIL';\"" >/dev/null

echo 'HIMATE START-23.11.3c production fixture readiness smoke passed'
