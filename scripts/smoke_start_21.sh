#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start21-owner.txt"
OPS_COOKIE="$TMP_ROOT/himate-start21-ops.txt"
FIN_COOKIE="$TMP_ROOT/himate-start21-fin.txt"
BODY="$TMP_ROOT/himate-start21-body.json"
rm -f "$OWNER_COOKIE" "$OPS_COOKIE" "$FIN_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$OPS_COOKIE" "$FIN_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
OPS_PASSWORD="$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"
FIN_PASSWORD="$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

login() {
  cookie="$1"; email="$2"; password="$3"
  payload="$(python3 - "$email" "$password" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login"
}

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-21 owner login and scoped administrators... '
login "$OWNER_COOKIE" "$OWNER_EMAIL" "$OWNER_PASSWORD" >/dev/null
ops_payload="$(python3 - "$OPS_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 21 Operations","email":"ci-start21-ops@example.com","password":sys.argv[1],"roles":["operations_admin"]}))
PY
)"
fin_payload="$(python3 - "$FIN_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 21 Finance","email":"ci-start21-fin@example.com","password":sys.argv[1],"roles":["finance_admin"]}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$ops_payload" "$BASE_URL/api/v1/admin/users" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$fin_payload" "$BASE_URL/api/v1/admin/users" >/dev/null
ops_login="$(login "$OPS_COOKIE" "ci-start21-ops@example.com" "$OPS_PASSWORD")"
login "$FIN_COOKIE" "ci-start21-fin@example.com" "$FIN_PASSWORD" >/dev/null
printf '%s' "$ops_login" | python3 -c 'import json,sys; d=json.load(sys.stdin); p=set(d["permissions"]); assert "*" in p or {"backups.read","backups.write","backups.approve"}.issubset(p),p'
echo ok

printf 'backup service is part of gateway and system health... '
gateway_health="$(curl -fsS "$BASE_URL/api/v1/health")"
printf '%s' "$gateway_health" | grep -q '"backups":"ok"'
system_health="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/system-health")"
printf '%s' "$system_health" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert any(x["name"]=="backups" and x["status"]=="OK" for x in d["services"]),d["services"]'
echo ok

printf 'reuse a physically provisioned partner database... '
db_name="$(docker compose exec -T postgres psql -U himate -d postgres -Atc "SELECT datname FROM pg_database WHERE datname ~ '^himate_ptr_[0-9]{6}$' ORDER BY datname LIMIT 1")"
test -n "$db_name"
partner_id="${db_name#himate_}"
curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_id" >/dev/null
identity="$(docker compose exec -T postgres psql -U himate -d "$db_name" -Atc "SELECT value FROM partner_core.system_meta WHERE key='partner_id'")"
test "$identity" = "$partner_id"
echo "$partner_id"

printf 'seed partner media for backup verification... '
media_marker="START21-MEDIA-${partner_id}-ENCRYPTED"
docker compose exec -T storage sh -c "mkdir -p '/data/partners/$partner_id/start21-smoke' && printf '%s' '$media_marker' > '/data/partners/$partner_id/start21-smoke/marker.txt' && chown himate:himate '/data/partners/$partner_id/start21-smoke/marker.txt' && chmod 600 '/data/partners/$partner_id/start21-smoke/marker.txt'"
echo ok

printf 'RBAC blocks Finance and permits Operations backup access... '
test "$(status "$FIN_COOKIE" GET "/api/v1/backups/summary")" = "403"
test "$(status "$FIN_COOKIE" POST "/api/v1/backups" -H 'Content-Type: application/json' -d "{\"partner_id\":\"$partner_id\"}")" = "403"
curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/backups/summary" >/dev/null
echo ok

printf 'queue encrypted offsite restore point... '
queued="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d "{\"partner_id\":\"$partner_id\"}" "$BASE_URL/api/v1/backups")"
point_id="$(printf '%s' "$queued" | json_field id)"
printf '%s' "$queued" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"] in ("QUEUED","RUNNING"); assert d["provider"]=="local"; assert d["partner_id"]==sys.argv[1]' "$partner_id"
test -n "$point_id"
echo "$point_id"

printf 'wait for database/media/config restore point... '
point_status=""
i=0
while [ "$i" -lt 120 ]; do
  point="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/backups/restore-points/$point_id")"
  point_status="$(printf '%s' "$point" | json_field status)"
  if [ "$point_status" = "READY" ]; then break; fi
  if [ "$point_status" = "FAILED" ]; then
    printf '%s\n' "$point"
    exit 1
  fi
  i=$((i+1))
  sleep 1
done
test "$point_status" = "READY"
printf '%s' "$point" | python3 -c 'import json,sys; d=json.load(sys.stdin); c=d["manifest"]["components"]; assert set(c)=={"database","media","config"}; assert all(len(c[x]["sha256"])==64 for x in c); assert all(c[x]["bytes"]>0 for x in c); assert d["ciphertext_bytes"]>0; assert len(d["ciphertext_sha256"])==64'
docker compose exec -T backups sh -c "test -s '/offsite/$partner_id/$point_id.hmbk'"
if docker compose exec -T backups sh -c "grep -a -F '$media_marker' '/offsite/$partner_id/$point_id.hmbk' >/dev/null"; then
  echo 'media marker leaked into encrypted offsite artifact'
  exit 1
fi
echo ok

printf 'wait for mandatory automatic restore verification... '
automatic_status=""
automatic_test=""
i=0
while [ "$i" -lt 120 ]; do
  tests="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/backups/restore-tests?restore_point_id=$point_id&limit=10")"
  automatic_status="$(printf '%s' "$tests" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["items"][0]["status"] if d["items"] else "")')"
  if [ "$automatic_status" = "PASSED" ]; then
    automatic_test="$(printf '%s' "$tests" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)["items"][0]))')"
    break
  fi
  if [ "$automatic_status" = "FAILED" ]; then
    printf '%s\n' "$tests"
    exit 1
  fi
  i=$((i+1))
  sleep 1
done
test "$automatic_status" = "PASSED"
printf '%s' "$automatic_test" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["database_ok"] is True; assert d["media_ok"] is True; assert d["config_ok"] is True; assert d["created_by"]=="automatic"; assert d["duration_ms"]>=0'
scratch_count="$(docker compose exec -T postgres psql -U himate -d postgres -Atc "SELECT COUNT(*) FROM pg_database WHERE datname LIKE 'himate_restore_%'")"
test "$scratch_count" = "0"
echo ok

printf 'backup summary reports verified recoverability... '
summary="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/backups/summary")"
printf '%s' "$summary" | python3 -c 'import json,sys; d=json.load(sys.stdin); pid=sys.argv[1]; item=next(x for x in d["items"] if x["partner_id"]==pid); assert item["latest_restore_point_id"]==sys.argv[2]; assert item["latest_backup_status"]=="READY"; assert item["latest_restore_test_status"]=="PASSED"; assert item["recoverability_status"]=="VERIFIED"; assert item["provider"]=="local"' "$partner_id" "$point_id"
echo ok

printf 'partner backup policy persists retention and schedule... '
policy="$(curl -fsS -b "$OPS_COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"retention_days":14,"max_restore_points":3,"schedule_hours":24,"enabled":true}' "$BASE_URL/api/v1/backups/policies/$partner_id")"
printf '%s' "$policy" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["retention_days"]==14; assert d["max_restore_points"]==3; assert d["schedule_hours"]==24; assert d["enabled"] is True'
curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/backups/policies/$partner_id" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["retention_days"]==14; assert d["max_restore_points"]==3'
echo ok

printf 'explicit restore-test endpoint restores from offsite copy... '
manual="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/backups/restore-points/$point_id/restore-test")"
manual_id="$(printf '%s' "$manual" | json_field id)"
test -n "$manual_id"
manual_status=""
i=0
while [ "$i" -lt 120 ]; do
  manual_test="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/backups/restore-tests/$manual_id")"
  manual_status="$(printf '%s' "$manual_test" | json_field status)"
  if [ "$manual_status" = "PASSED" ]; then break; fi
  if [ "$manual_status" = "FAILED" ]; then
    printf '%s\n' "$manual_test"
    exit 1
  fi
  i=$((i+1))
  sleep 1
done
test "$manual_status" = "PASSED"
printf '%s' "$manual_test" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["database_ok"] and d["media_ok"] and d["config_ok"]'
test "$(docker compose exec -T postgres psql -U himate -d postgres -Atc "SELECT COUNT(*) FROM pg_database WHERE datname LIKE 'himate_restore_%'")" = "0"
echo ok

printf 'retention prune removes expired offsite artifact... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE backups.restore_points SET expires_at=NOW()-INTERVAL '1 hour' WHERE id='$point_id'" >/dev/null
pruned="$(curl -fsS -b "$OPS_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/backups/prune?partner_id=$partner_id")"
printf '%s' "$pruned" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["expired"]>=1'
expired="$(curl -fsS -b "$OPS_COOKIE" "$BASE_URL/api/v1/backups/restore-points/$point_id")"
printf '%s' "$expired" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="EXPIRED"; assert d["object_key"]==""'
if docker compose exec -T backups sh -c "test -e '/offsite/$partner_id/$point_id.hmbk'"; then
  echo 'expired offsite artifact still exists'
  exit 1
fi
echo ok

printf 'START-21 mutations are present in central audit... '
sleep 1
audit="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/audit/events?q=BACKUP_&limit=100")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); actions={x["action"] for x in d["items"] if x["resource"]=="backups" and x["outcome"]=="SUCCESS"}; required={"BACKUP_RESTORE_POINT_QUEUED","BACKUP_POLICY_UPDATED","BACKUP_RESTORE_TEST_QUEUED","BACKUP_RETENTION_PRUNED"}; assert required.issubset(actions),(required,actions)'
echo ok

echo "HIMATE START-21 encrypted backups, offsite retention and verified recovery smoke passed"
