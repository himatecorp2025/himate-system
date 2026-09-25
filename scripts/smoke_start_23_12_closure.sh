#!/bin/sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
AUTOMATION_URL="${2:-http://127.0.0.1:18082}"
TENANT_FINANCE_URL="${3:-http://127.0.0.1:18083}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-2312-closure-owner.txt"
BODY="$TMP_ROOT/himate-2312-closure-body.json"
rm -f "$OWNER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
env_value() {
  service="$1"
  key="$2"
  printf '%s' "$COMPOSE_JSON" | python3 - "$service" "$key" <<'PY'
import json,sys
service,key=sys.argv[1:]
d=json.load(sys.stdin)
e=d["services"][service]["environment"]
if isinstance(e,dict):
    print(e[key])
else:
    print(next(x.split("=",1)[1] for x in e if x.startswith(key+"=")))
PY
}

CLOSURE_INTERNAL_TOKEN="$(env_value automation HIMATE_INTERNAL_TOKEN)"
AUTOMATION_KEYS="$(env_value automation HIMATE_AUTOMATION_SERVICE_KEYS_JSON)"
OWNER_EMAIL="$(env_value gateway HIMATE_BOOTSTRAP_ADMIN_EMAIL)"
OWNER_PASSWORD="$(env_value gateway HIMATE_BOOTSTRAP_ADMIN_PASSWORD)"

automation_secret() {
  printf '%s' "$AUTOMATION_KEYS" | python3 - "$1" <<'PY'
import json,sys
print(json.load(sys.stdin)[sys.argv[1]])
PY
}

echo "recreate private topology with production service-signature enforcement..."
HIMATE_REQUIRE_SERVICE_SIGNATURE=true docker compose up -d --force-recreate

i=0
while [ "$i" -lt 90 ]; do
  if curl -fsS "$BASE_URL/api/v1/health" >/dev/null 2>&1 &&
     curl -fsS "${AUTOMATION_URL%/}/health" >/dev/null 2>&1 &&
     curl -fsS "${TENANT_FINANCE_URL%/}/health" >/dev/null 2>&1; then
    break
  fi
  i=$((i+1))
  sleep 2
done
if [ "$i" -ge 90 ]; then
  docker compose ps
  docker compose logs --no-color
  exit 1
fi

printf 'unsigned internal traffic is rejected under production service identity... '
code="$(curl -sS -o "$BODY" -w '%{http_code}' -X POST   -H "X-Himate-Internal-Token: $CLOSURE_INTERNAL_TOKEN"   -H 'Content-Type: application/json' -d '{"limit":1}'   "${AUTOMATION_URL%/}/internal/v1/automation/deliveries/claim")"
test "$code" = "403"
grep -q 'SERVICE_IDENTITY' "$BODY"
echo ok

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'Gateway still reaches signed Catalog routes... '
curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules" >/dev/null
echo ok

STAMP="$(date +%s)"
partner_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({
  "display_name":"START 23.12 Closure "+s,
  "legal_name":"START 23.12 Closure LLC "+s,
  "brand_name":"Closure "+s,
  "contact_name":"Closure Owner",
  "contact_email":"closure-"+s+"@example.test",
  "country":"US"
}))
PY
)"
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
PARTNER_ID="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"

printf 'Workshop passes both signature layers but finance fails closed without invoice entitlement... '
CLOSURE_AUTOMATION_SECRET="$(automation_secret workshop)" CLOSURE_INTERNAL_TOKEN="$CLOSURE_INTERNAL_TOKEN" python3 scripts/start_23_12_closure_publish.py   "$AUTOMATION_URL" "$PARTNER_ID" "$STAMP" workshop workflow.billing_approved.v1 blocked >/dev/null

i=0
while [ "$i" -lt 20 ]; do
  last_error="$(docker compose exec -T postgres psql -U himate -d himate -Atqc "SELECT COALESCE(MAX(d.last_error),'') FROM automation.deliveries d JOIN automation.events e ON e.id=d.event_id WHERE e.event_key='closure-workshop-blocked-$STAMP'")"
  invoice_count="$(docker compose exec -T postgres psql -U himate -d himate -Atqc "SELECT COUNT(*) FROM tenant_finance.invoices WHERE partner_id='$PARTNER_ID' AND source_id='blocked-$STAMP'")"
  if printf '%s' "$last_error" | grep -q 'invoice_documents organization entitlement is not ACTIVE and executable' && [ "$invoice_count" = "0" ]; then
    break
  fi
  i=$((i+1))
  sleep 1
done
test "$i" -lt 20
echo ok

printf 'activate authoritative invoice_documents organization entitlement... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 <<SQL >/dev/null
UPDATE catalog.modules
SET publication_status='PUBLISHED',implementation_state='READY',availability='ACTIVE'
WHERE module_key='invoice_documents';

INSERT INTO catalog.partner_modules(
  partner_id,module_key,status,visible,included_in_base,entitlement_state,
  commercial_configured,contract_currency,quote_reference,commercial_effective_at,
  entitlement_source,activated_at
) VALUES (
  '$PARTNER_ID','invoice_documents','ACTIVE',TRUE,TRUE,'ACTIVE',
  TRUE,'USD','START-23.12-CLOSURE',NOW(),'MANUAL',NOW()
)
ON CONFLICT(partner_id,module_key) DO UPDATE SET
  status='ACTIVE',visible=TRUE,included_in_base=TRUE,entitlement_state='ACTIVE',
  commercial_configured=TRUE,contract_currency='USD',quote_reference='START-23.12-CLOSURE',
  commercial_effective_at=NOW(),entitlement_source='MANUAL',
  activated_at=COALESCE(catalog.partner_modules.activated_at,NOW()),updated_at=NOW();
SQL
echo ok

printf 'Scheduler passes both signature layers and finance succeeds after entitlement... '
CLOSURE_AUTOMATION_SECRET="$(automation_secret scheduler)" CLOSURE_INTERNAL_TOKEN="$CLOSURE_INTERNAL_TOKEN" python3 scripts/start_23_12_closure_publish.py   "$AUTOMATION_URL" "$PARTNER_ID" "$STAMP" scheduler scheduler.job_closed_invoice_ready.v1 allowed >/dev/null

i=0
while [ "$i" -lt 30 ]; do
  invoice_count="$(docker compose exec -T postgres psql -U himate -d himate -Atqc "SELECT COUNT(*) FROM tenant_finance.invoices WHERE partner_id='$PARTNER_ID' AND source_type='SCHEDULE' AND source_id='allowed-$STAMP' AND status='READY_FOR_ISSUE'")"
  if [ "$invoice_count" = "1" ]; then
    break
  fi
  i=$((i+1))
  sleep 1
done
test "$i" -lt 30
echo ok

echo "HIMATE START-23.12 Phase 1-5 cross-phase production runtime closure: PASS"
