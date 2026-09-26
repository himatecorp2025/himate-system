#!/usr/bin/env sh
set -eu

BASE_URL="$1"
if [ -z "$BASE_URL" ]; then BASE_URL="http://127.0.0.1:8080"; fi
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-central11-owner.txt"
rm -f "$COOKIE"
trap 'rm -f "$COOKIE"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'CENTRAL-11 health gate... '
curl -fsS "$BASE_URL/api/v1/health" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="ok",d'
echo ok

STAMP="$(date +%s)"
DISPLAY="Central 11 Test Partner $STAMP"
PARTNER_PAYLOAD="$(python3 - "$STAMP" "$DISPLAY" <<'PY'
import json,sys
stamp,display=sys.argv[1:]
print(json.dumps({
  "display_name":display,
  "legal_name":"Central 11 Test Partner LLC "+stamp,
  "brand_name":"Central 11 QA "+stamp,
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "contact_name":"Central 11 QA Owner",
  "contact_email":"central11."+stamp+"@himate.test",
  "registration_number":"CENTRAL11-"+stamp,
  "tax_id":"CENTRAL11-TAX-"+stamp,
  "country":"United States",
  "state_region":"New York",
  "city":"New York",
  "notes":"CENTRAL-11 synthetic partner fixture"
}))
PY
)"

printf 'create and promote Golden Test Partner... '
PARTNER="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$PARTNER_PAYLOAD" "$BASE_URL/api/v1/partners")"
PARTNER_ID="$(printf '%s' "$PARTNER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
PROMOTED="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"CENTRAL-11 fixture acceptance"}' "$BASE_URL/api/v1/partners/$PARTNER_ID")"
printf '%s' "$PROMOTED" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["test_partner"] is True,d; assert d["lifecycle"]=="LIVE",d'
echo ok

printf 'capture aggregate finance ledger before fixture... '
BEFORE_FINANCE="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/finance/overview")"
BEFORE_CANON="$(printf '%s' "$BEFORE_FINANCE" | python3 -c 'import json,sys; d=json.load(sys.stdin); keys=["currencies","monthly_paid","weekly_paid","monthly_paid_by_plan","weekly_paid_by_plan"]; print(json.dumps({k:d[k] for k in keys},sort_keys=True,separators=(",",":")))' )"
echo ok

printf 'generate six-month piano-service fixture... '
FIXTURE="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/partners/$PARTNER_ID/seed-test-fixture")"
REPORT_ID="$(printf '%s' "$FIXTURE" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["months"]==6,d; assert d["aggregate_isolation"] is True,d; assert d["fixture"]=="HIMATE_GOLDEN_TEST_FIXTURE",d; print(d["report_id"])')"
echo ok

printf 'verify fixture cardinality in PostgreSQL... '
COUNTS="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' -v partner_id="$PARTNER_ID" -v report_id="$REPORT_ID" <<'SQL'
SELECT
  (SELECT COUNT(*) FROM billing.invoices WHERE partner_id=:'partner_id' AND source='TEST_FIXTURE'),
  (SELECT COUNT(*) FROM impact.metric_values WHERE partner_id=:'partner_id' AND provenance='TEST_FIXTURE'),
  (SELECT COUNT(*) FROM evidence.items WHERE partner_id=:'partner_id' AND description LIKE 'HIMATE_GOLDEN_TEST_FIXTURE%'),
  (SELECT COUNT(*) FROM reports.jobs WHERE id=:'report_id' AND partner_ids @> ('["' || :'partner_id' || '"]')::jsonb);
SQL
)"
test "$COUNTS" = "6|18|6|1"
echo ok

printf 'partner-specific Impact exposes QA metrics... '
IMPACT="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/impact/summary?partner_id=$PARTNER_ID")"
printf '%s' "$IMPACT" | python3 -c 'import json,sys; d=json.load(sys.stdin); keys={x["metric_key"] for x in d["items"]}; required={"qa.piano.pianos_serviced","qa.piano.technician_hours","qa.piano.customer_jobs"}; assert required.issubset(keys),(required,keys)'
echo ok

printf 'wait for normal Reports worker to produce the real PDF... '
REPORT_STATUS=""
i=0
while [ "$i" -lt 30 ]; do
  REPORT="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/reports/$REPORT_ID")"
  REPORT_STATUS="$(printf '%s' "$REPORT" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')"
  if [ "$REPORT_STATUS" = "READY" ]; then break; fi
  if [ "$REPORT_STATUS" = "FAILED" ]; then
    printf '%s
' "$REPORT" >&2
    exit 1
  fi
  i=$((i+1))
  sleep 1
done
test "$REPORT_STATUS" = "READY"
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/reports/$REPORT_ID/download" -o "$TMP_ROOT/central11-test-report.pdf"
test -s "$TMP_ROOT/central11-test-report.pdf"
rm -f "$TMP_ROOT/central11-test-report.pdf"
echo ok

printf 'capture generated report storage location... '
REPORT_OBJECT="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' -v report_id="$REPORT_ID" <<'SQL'
SELECT pdf_namespace,pdf_object_key FROM reports.jobs WHERE id=:'report_id';
SQL
)"
REPORT_NAMESPACE="$(printf '%s' "$REPORT_OBJECT" | cut -d'|' -f1)"
REPORT_KEY="$(printf '%s' "$REPORT_OBJECT" | cut -d'|' -f2-)"
test "$REPORT_NAMESPACE" = "_reports"
test -n "$REPORT_KEY"
docker compose exec -T storage sh -c "test -f '/data/partners/_reports/$REPORT_KEY'"
echo ok

printf 'fixture remains isolated from platform finance aggregates... '
AFTER_FINANCE="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/finance/overview")"
AFTER_CANON="$(printf '%s' "$AFTER_FINANCE" | python3 -c 'import json,sys; d=json.load(sys.stdin); keys=["currencies","monthly_paid","weekly_paid","monthly_paid_by_plan","weekly_paid_by_plan"]; print(json.dumps({k:d[k] for k in keys},sort_keys=True,separators=(",",":")))' )"
test "$BEFORE_CANON" = "$AFTER_CANON"
echo ok

printf 'factory-reset requires exact confirmation... '
BAD_STATUS="$(curl -sS -o "$TMP_ROOT/central11-bad-purge.json" -w '%{http_code}' -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{"confirm_partner_id":"WRONG"}' "$BASE_URL/api/v1/partners/$PARTNER_ID/purge-test-fixture")"
test "$BAD_STATUS" = "400"
rm -f "$TMP_ROOT/central11-bad-purge.json"
echo ok

printf 'factory-reset Golden Test Partner completely... '
PURGE_PAYLOAD="$(python3 - "$PARTNER_ID" <<'PY'
import json,sys
print(json.dumps({"confirm_partner_id":sys.argv[1]}))
PY
)"
PURGE="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d "$PURGE_PAYLOAD" "$BASE_URL/api/v1/partners/$PARTNER_ID/purge-test-fixture")"
printf '%s' "$PURGE" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["hard_purged"] is True,d; assert d["compliance_archive_created"] is False,d; assert d["audit_receipt_preserved"] is True,d'
echo ok

printf 'verify relational and persistent-storage cleanup... '
LEFT="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' -v partner_id="$PARTNER_ID" -v report_id="$REPORT_ID" <<'SQL'
SELECT
  (SELECT COUNT(*) FROM partners.partners WHERE id=:'partner_id'),
  (SELECT COUNT(*) FROM billing.invoices WHERE partner_id=:'partner_id'),
  (SELECT COUNT(*) FROM impact.metric_values WHERE partner_id=:'partner_id'),
  (SELECT COUNT(*) FROM evidence.items WHERE partner_id=:'partner_id'),
  (SELECT COUNT(*) FROM catalog.partner_modules WHERE partner_id=:'partner_id'),
  (SELECT COUNT(*) FROM reports.jobs WHERE id=:'report_id'),
  (SELECT COUNT(*) FROM storage.partner_namespaces WHERE partner_id=:'partner_id'),
  (SELECT COUNT(*) FROM compliance.partner_archives WHERE partner_id=:'partner_id');
SQL
)"
test "$LEFT" = "0|0|0|0|0|0|0|0"
docker compose exec -T storage sh -c "test ! -e '/data/partners/$PARTNER_ID'"
docker compose exec -T storage sh -c "test ! -e '/data/partners/_reports/$REPORT_KEY'"
echo ok

printf 'security audit receipt survives business-data purge... '
AUDIT_COUNT="$(docker compose exec -T postgres psql -U himate -d himate -At -v partner_id="$PARTNER_ID" <<'SQL'
SELECT COUNT(*) FROM identity.audit_events
WHERE partner_id=:'partner_id'
  AND path LIKE '%/purge-test-fixture'
  AND outcome='SUCCESS';
SQL
)"
test "$AUDIT_COUNT" -ge 1
echo ok

printf 'partner is no longer addressable... '
STATUS="$(curl -sS -o /dev/null -w '%{http_code}' -b "$COOKIE" "$BASE_URL/api/v1/partners/$PARTNER_ID")"
test "$STATUS" = "404"
echo ok

echo 'CENTRAL-11 runtime UX, fixture, aggregate-isolation and factory-reset acceptance passed'
