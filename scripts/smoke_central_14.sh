#!/usr/bin/env sh
set -eu

BASE_URL="http://127.0.0.1:8080"
if [ "$#" -ge 1 ] && [ -n "$1" ]; then BASE_URL="$1"; fi
TMP_ROOT="/tmp"
COOKIE="$TMP_ROOT/himate-central14-owner.txt"
BODY="$TMP_ROOT/himate-central14-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin).get(sys.argv[1],""))' "$1"
}

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
LOGIN="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$LOGIN" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'CENTRAL-14 health gate... '
curl -fsS "$BASE_URL/api/v1/health" | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"]=="ok"'
echo ok

printf 'HIMATE platform backup policy exists and is scheduled... '
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/backups/policies/_platform" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]=="_platform",d; assert d["enabled"] is True,d; assert d["schedule_hours"]==24,d'
echo ok

printf 'wait for verified HIMATE platform restore point... '
PLATFORM_POINT=""
i=0
while [ "$i" -lt 180 ]; do
  SUMMARY="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/backups/summary")"
  PLATFORM_STATE="$(printf '%s' "$SUMMARY" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next((x for x in d["items"] if x["partner_id"]=="_platform"),{}); print(x.get("recoverability_status",""))')"
  PLATFORM_POINT="$(printf '%s' "$SUMMARY" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next((x for x in d["items"] if x["partner_id"]=="_platform"),{}); print(x.get("latest_restore_point_id",""))')"
  if [ "$PLATFORM_STATE" = "VERIFIED" ] && [ -n "$PLATFORM_POINT" ]; then break; fi
  if [ "$i" -eq 5 ]; then
    curl -sS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"partner_id":"_platform"}' "$BASE_URL/api/v1/backups" >/dev/null || true
  fi
  i=$((i+1))
  sleep 1
done
test "$PLATFORM_STATE" = "VERIFIED"
test -n "$PLATFORM_POINT"
echo ok

printf 'live platform replacement is maintenance-only... '
PLATFORM_RESTORE_PAYLOAD="$(python3 - <<'PY'
import json
print(json.dumps({"confirmation":"RESTORE _platform","reason":"CENTRAL-14 platform safety gate acceptance"}))
PY
)"
PLATFORM_STATUS="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d "$PLATFORM_RESTORE_PAYLOAD" "$BASE_URL/api/v1/backups/restore-points/$PLATFORM_POINT/restore?partner_id=_platform")"
test "$PLATFORM_STATUS" = "409"
grep -qi 'maintenance-only' "$BODY"
echo ok

printf 'reuse a physically provisioned non-archived partner database... '
PARTNER_ID="$(docker compose exec -T postgres psql -U himate -d himate -At <<'SQL'
SELECT p.id
FROM partners.partners p
WHERE p.lifecycle<>'ARCHIVED'
  AND EXISTS(SELECT 1 FROM pg_database d WHERE d.datname='himate_'||p.id)
ORDER BY CASE WHEN p.id='ptr_000001' THEN 1 ELSE 0 END,p.id
LIMIT 1;
SQL
)"
test -n "$PARTNER_ID"
DB_NAME="himate_$PARTNER_ID"
PARTNER_BEFORE="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$PARTNER_ID")"
echo "$PARTNER_ID"

BASELINE_NAME="Central 14 Recovery Baseline $STAMP"
MUTATED_NAME="Central 14 Recovery Mutated $STAMP"
BASELINE_DB="CENTRAL14-DB-BASELINE-$STAMP"
MUTATED_DB="CENTRAL14-DB-MUTATED-$STAMP"
BASELINE_MEDIA="CENTRAL14-MEDIA-BASELINE-$STAMP"
MUTATED_MEDIA="CENTRAL14-MEDIA-MUTATED-$STAMP"

printf 'suspend partner and seed baseline master/database/media state... '
BASELINE_PATCH="$(python3 - "$BASELINE_NAME" <<'PY'
import json,sys
print(json.dumps({"display_name":sys.argv[1],"lifecycle":"SUSPENDED","reason":"CENTRAL-14 verified recovery acceptance"}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$BASELINE_PATCH" "$BASE_URL/api/v1/partners/$PARTNER_ID" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle"]=="SUSPENDED",d'
docker compose exec -T postgres psql -U himate -d "$DB_NAME" -v ON_ERROR_STOP=1 -v marker="$BASELINE_DB" <<'SQL' >/dev/null
INSERT INTO partner_core.system_meta(key,value)
VALUES('central14_restore_marker',:'marker')
ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value;
SQL
docker compose exec -T storage sh -c "mkdir -p '/data/partners/$PARTNER_ID/central14' && printf '%s' '$BASELINE_MEDIA' > '/data/partners/$PARTNER_ID/central14/marker.txt' && chown himate:himate '/data/partners/$PARTNER_ID/central14/marker.txt' && chmod 600 '/data/partners/$PARTNER_ID/central14/marker.txt'"
echo ok

printf 'register and search HIMATE corporate administration document... '
COMPANY_DOC_NAME="CENTRAL14 Governance Policy $STAMP"
COMPANY_DOC_PAYLOAD="$(python3 - "$COMPANY_DOC_NAME" <<'PY'
import json,sys
print(json.dumps({"kind":"GOVERNANCE","name":sys.argv[1],"storage_url":"ci://central14/company","note":"CENTRAL-14 corporate administration acceptance"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$COMPANY_DOC_PAYLOAD" "$BASE_URL/api/v1/billing/company/documents" >/dev/null
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/company/documents?q=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))' "$COMPANY_DOC_NAME")" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert len(d["items"])==1,d; assert d["items"][0]["kind"]=="GOVERNANCE",d'
echo ok

printf 'register and search partner administration document... '
PARTNER_DOC_NAME="CENTRAL14 Partner Administration $STAMP"
PARTNER_DOC_PAYLOAD="$(python3 - "$PARTNER_DOC_NAME" <<'PY'
import json,sys
print(json.dumps({"kind":"ADMINISTRATION","name":sys.argv[1],"storage_url":"ci://central14/partner","note":"CENTRAL-14 tenant administration acceptance"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$PARTNER_DOC_PAYLOAD" "$BASE_URL/api/v1/billing/partners/$PARTNER_ID/documents" >/dev/null
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$PARTNER_ID/documents?q=CENTRAL14" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert any(x["kind"]=="ADMINISTRATION" for x in d["items"]),d'
echo ok

printf 'queue target partner restore point... '
TARGET_PAYLOAD="$(python3 - "$PARTNER_ID" <<'PY'
import json,sys
print(json.dumps({"partner_id":sys.argv[1]}))
PY
)"
TARGET="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$TARGET_PAYLOAD" "$BASE_URL/api/v1/backups")"
TARGET_POINT="$(printf '%s' "$TARGET" | json_field id)"
test -n "$TARGET_POINT"
echo "$TARGET_POINT"

printf 'wait for target restore point and real restore test... '
i=0
TARGET_RECOVERY=""
while [ "$i" -lt 180 ]; do
  POINT="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/backups/restore-points/$TARGET_POINT")"
  POINT_STATUS="$(printf '%s' "$POINT" | json_field status)"
  if [ "$POINT_STATUS" = "FAILED" ]; then printf '%s
' "$POINT"; exit 1; fi
  SUMMARY="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/backups/summary")"
  TARGET_RECOVERY="$(printf '%s' "$SUMMARY" | python3 -c 'import json,sys; d=json.load(sys.stdin); pid,point=sys.argv[1:]; x=next((x for x in d["items"] if x["partner_id"]==pid),{}); print(x.get("recoverability_status","") if x.get("latest_restore_point_id","")==point else "")' "$PARTNER_ID" "$TARGET_POINT")"
  if [ "$POINT_STATUS" = "READY" ] && [ "$TARGET_RECOVERY" = "VERIFIED" ]; then break; fi
  i=$((i+1))
  sleep 1
done
test "$POINT_STATUS" = "READY"
test "$TARGET_RECOVERY" = "VERIFIED"
echo ok

printf 'Administration read model composes company + tenant domains... '
ADMIN="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/administration?q=$PARTNER_ID&limit=200&offset=0")"
printf '%s' "$ADMIN" | python3 -c 'import json,sys; d=json.load(sys.stdin); pid=sys.argv[1]; assert d["meta"]["architecture"]=="GO_BACKEND_READ_MODEL",d; assert d["company"]["recoverability_status"]=="VERIFIED",d["company"]; rows=[x for x in d["items"] if x["partner_id"]==pid]; assert len(rows)==1,(pid,d); row=rows[0]; assert row["document_count"]>=1,row; assert row["recoverability_status"]=="VERIFIED",row' "$PARTNER_ID"
echo ok

printf 'mutate master/database/media after backup... '
MUTATION_PATCH="$(python3 - "$MUTATED_NAME" <<'PY'
import json,sys
print(json.dumps({"display_name":sys.argv[1],"reason":"CENTRAL-14 post-backup mutation"}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$MUTATION_PATCH" "$BASE_URL/api/v1/partners/$PARTNER_ID" >/dev/null
docker compose exec -T postgres psql -U himate -d "$DB_NAME" -v ON_ERROR_STOP=1 -v marker="$MUTATED_DB" <<'SQL' >/dev/null
UPDATE partner_core.system_meta SET value=:'marker' WHERE key='central14_restore_marker';
SQL
docker compose exec -T storage sh -c "printf '%s' '$MUTATED_MEDIA' > '/data/partners/$PARTNER_ID/central14/marker.txt'"
echo ok

printf 'queue verified production restore... '
RESTORE_PAYLOAD="$(python3 - "$PARTNER_ID" <<'PY'
import json,sys
print(json.dumps({"confirmation":"RESTORE "+sys.argv[1],"reason":"CENTRAL-14 end-to-end verified recovery acceptance"}))
PY
)"
RESTORE_JOB="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d "$RESTORE_PAYLOAD" "$BASE_URL/api/v1/backups/restore-points/$TARGET_POINT/restore?partner_id=$PARTNER_ID")"
RESTORE_JOB_ID="$(printf '%s' "$RESTORE_JOB" | json_field id)"
test -n "$RESTORE_JOB_ID"
echo "$RESTORE_JOB_ID"

printf 'wait for production restore completion and safety backup... '
i=0
RESTORE_STATUS=""
while [ "$i" -lt 240 ]; do
  JOB="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/backups/restores/$RESTORE_JOB_ID")"
  RESTORE_STATUS="$(printf '%s' "$JOB" | json_field status)"
  if [ "$RESTORE_STATUS" = "COMPLETED" ]; then break; fi
  if [ "$RESTORE_STATUS" = "FAILED" ]; then printf '%s
' "$JOB"; exit 1; fi
  i=$((i+1))
  sleep 2
done
test "$RESTORE_STATUS" = "COMPLETED"
printf '%s' "$JOB" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["database_ok"] and d["media_ok"] and d["config_ok"],d; assert d["safety_restore_point_id"],d; assert d["error"]=="",d'
echo ok

printf 'database, media and partner master state returned to target snapshot... '
RESTORED="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$PARTNER_ID")"
printf '%s' "$RESTORED" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["display_name"]==sys.argv[1],d; assert d["lifecycle"]=="SUSPENDED",d' "$BASELINE_NAME"
DB_VALUE="$(docker compose exec -T postgres psql -U himate -d "$DB_NAME" -Atc "SELECT value FROM partner_core.system_meta WHERE key='central14_restore_marker'")"
test "$DB_VALUE" = "$BASELINE_DB"
MEDIA_VALUE="$(docker compose exec -T storage sh -c "cat '/data/partners/$PARTNER_ID/central14/marker.txt'")"
test "$MEDIA_VALUE" = "$BASELINE_MEDIA"
echo ok

printf 'restored runtimes remain suspended and captured active releases are preserved... '
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/environments?partner_id=$PARTNER_ID" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert all(x["environment_status"]=="SUSPENDED" for x in d.get("items",[])),d'
echo ok

printf 'production restore request is immutable-audit visible... '
sleep 1
AUDIT="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/audit/events?partner_id=$PARTNER_ID&q=BACKUP_PRODUCTION_RESTORE_QUEUED&limit=100")"
printf '%s' "$AUDIT" | python3 -c 'import json,sys; d=json.load(sys.stdin); pid=sys.argv[1]; rows=[x for x in d["items"] if x["action"]=="BACKUP_PRODUCTION_RESTORE_QUEUED" and x["partner_id"]==pid and x["outcome"]=="SUCCESS"]; assert rows,(pid,d)' "$PARTNER_ID"
echo ok

printf 'post-restore administration read model remains tenant-scoped and recoverable... '
i=0
while [ "$i" -lt 120 ]; do
  ADMIN="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/administration?q=$PARTNER_ID&limit=200&offset=0")"
  PARTNER_RECOVERY="$(printf '%s' "$ADMIN" | python3 -c 'import json,sys; d=json.load(sys.stdin); pid=sys.argv[1]; rows=[x for x in d["items"] if x["partner_id"]==pid]; print(rows[0].get("recoverability_status","") if rows else "")' "$PARTNER_ID")"
  if [ "$PARTNER_RECOVERY" = "VERIFIED" ]; then break; fi
  i=$((i+1))
  sleep 1
done
test "$PARTNER_RECOVERY" = "VERIFIED"
echo ok

echo 'CENTRAL-14 Administration, Backup & Recovery runtime acceptance passed'
