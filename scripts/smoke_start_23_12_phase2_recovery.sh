#!/usr/bin/env sh
set -eu
BASE_URL="${1:-http://127.0.0.1:8080}"
STAMP="$(date +%s)"
AUDIT_REQUEST="phase2-restart-$STAMP"
SAGA_REQUEST="phase2-saga-restart-$STAMP"
SAGA_EMAIL="phase2-saga-owner-$STAMP@himate.test"
JOB_ID="prv_phase2_recovery_$STAMP"
PARTNER_ID="ptr_phase2_recovery_$STAMP"
TODAY="$(date -u +%Y-%m-%d)"

OWNER_ID="$(docker compose exec -T postgres psql -U himate -d himate -At -c "SELECT id FROM identity.users WHERE system_owner=TRUE AND active=TRUE ORDER BY id LIMIT 1;")"
test -n "$OWNER_ID"

printf 'prepare a PENDING Partner onboarding saga before gateway restart... '
docker compose exec -T postgres psql -U himate -d himate -v req="$SAGA_REQUEST" -v actor="$OWNER_ID" -v email="$SAGA_EMAIL" -v stamp="$STAMP" -v today="$TODAY" >/dev/null <<'SQL'
INSERT INTO identity.partner_onboarding_sagas(
 request_id,actor_id,status,partner_payload,portal_owner_name,portal_owner_email,portal_owner_password_hash,billing_terms
) VALUES(
 :'req',:'actor','PENDING',
 jsonb_build_object(
  'display_name','Phase 2 Restart Partner '||:'stamp',
  'legal_name','Phase 2 Restart Partner LLC '||:'stamp',
  'category_id','cat_005','lifecycle','PROSPECT',
  'contact_name','Phase 2 Restart Owner','contact_email',:'email',
  'country','United States'
 ),
 'Phase 2 Restart Owner',:'email','phase2-test-hash',
 jsonb_build_object(
  'currency','USD','activation_fee',0,'activation_fee_waived',true,
  'activation_fee_reason','Phase 2 restart recovery',
  'base_monthly_fee',1500,'minimum_monthly_commitment',1500,
  'quote_reference','P2-RECOVERY-'||:'stamp','annual_increase_percent',10,
  'price_effective_from',:'today','service_anchor_date',:'today',
  'reason','START-23.12 Phase 2 restart recovery'
 )
);
SQL
echo ok

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

printf 'onboarding recovery worker completes the persisted saga... '
i=0
while :; do
  SAGA_STATUS="$(docker compose exec -T postgres psql -U himate -d himate -At -v req="$SAGA_REQUEST" -c "SELECT status FROM identity.partner_onboarding_sagas WHERE request_id=:'req';")"
  [ "$SAGA_STATUS" = "COMPLETE" ] && break
  i=$((i+1)); test "$i" -lt 45; sleep 1
done
SAGA_PARTNER_COUNT="$(docker compose exec -T postgres psql -U himate -d himate -At -v req="$SAGA_REQUEST" -c "SELECT COUNT(*) FROM partners.partners WHERE onboarding_request_id=:'req';")"
test "$SAGA_PARTNER_COUNT" = "1"
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
