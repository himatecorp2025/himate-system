#!/usr/bin/env sh
set -eu
BASE_URL="${1:-http://127.0.0.1:8080}"
STAMP="$(date +%s)"
AUDIT_REQUEST="phase2-restart-$STAMP"
JOB_ID="prv_phase2_recovery_$STAMP"
PARTNER_ID="ptr_phase2_recovery_$STAMP"

printf 'gateway restart recovers durable audit outbox... '
docker compose exec -T postgres psql -U himate -d himate -v req="$AUDIT_REQUEST" >/dev/null <<'SQL'
INSERT INTO identity.audit_outbox(
 actor_id,actor_name,actor_roles,request_id,correlation_id,action,method,path,resource,old_state,request_state
) VALUES(
 'phase2-recovery','Phase 2 Recovery','["platform_admin"]'::jsonb,
 :'req','corr-'||:'req','PHASE2_RESTART_CHECK','POST','/phase2/restart','audit','{}'::jsonb,'{"restart":true}'::jsonb
);
SQL
docker compose restart gateway >/dev/null
i=0
until curl -fsS "$BASE_URL/api/v1/live" >/dev/null 2>&1; do
  i=$((i+1)); test "$i" -lt 60; sleep 1
done
OUTBOX_LEFT="$(docker compose exec -T postgres psql -U himate -d himate -At -v req="$AUDIT_REQUEST" -c "SELECT COUNT(*) FROM identity.audit_outbox WHERE request_id=:'req';")"
AUDIT_COUNT="$(docker compose exec -T postgres psql -U himate -d himate -At -v req="$AUDIT_REQUEST" -c "SELECT COUNT(*) FROM identity.audit_events WHERE request_id=:'req' AND outcome='INTERRUPTED';")"
test "$OUTBOX_LEFT" = "0"
test "$AUDIT_COUNT" = "1"
echo ok

printf 'provisioning restart recovers RUNNING durable job... '
docker compose exec -T postgres psql -U himate -d himate -v job="$JOB_ID" -v partner="$PARTNER_ID" >/dev/null <<'SQL'
BEGIN;
INSERT INTO provisioning.jobs(
 id,partner_id,system_name,admin_email,platform_version,desired_release,initial_environment,module_preset,status,current_step
) VALUES(
 :'job',:'partner','phase2-recovery','','phase2','phase2','STAGING','[]'::jsonb,'RUNNING','COMPLETE'
);
INSERT INTO provisioning.steps(job_id,step_key,status,completed_at)
SELECT :'job',step_key,'SUCCESS',NOW()
FROM (VALUES
 ('VALIDATE_PARTNER'),('VALIDATE_LICENSE'),('MARK_PROVISIONING'),('CREATE_DATABASE'),
 ('SEED_REFERENCE_TEMPLATE'),('APPLY_MODULE_PRESET'),('CREATE_STORAGE'),('CREATE_STAGING_ENVIRONMENT'),
 ('CREATE_CONNECTOR_CREDENTIAL'),('SYNC_DESIRED_STATE'),('DEPLOY_STAGING'),('STORAGE_HEALTH'),
 ('PARTNER_DATABASE_HEALTH'),('STAGING_RUNTIME_HEALTH'),('COMPLETE')
) AS steps(step_key);
COMMIT;
SQL
STEP_COUNT="$(docker compose exec -T postgres psql -U himate -d himate -At -v job="$JOB_ID" -c "SELECT COUNT(*) FROM provisioning.steps WHERE job_id=:'job';")"
test "$STEP_COUNT" = "15"
docker compose restart provisioning >/dev/null
i=0
while :; do
  STATUS="$(docker compose exec -T postgres psql -U himate -d himate -At -v job="$JOB_ID" -c "SELECT status FROM provisioning.jobs WHERE id=:'job';")"
  [ "$STATUS" = "CONFIGURATION_REQUIRED" ] && break
  i=$((i+1)); test "$i" -lt 45; sleep 1
done
echo ok

echo 'HIMATE START-23.12 Phase 2 restart recovery smoke passed'
