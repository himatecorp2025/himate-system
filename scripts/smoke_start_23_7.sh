#!/usr/bin/env sh
set -eu

. scripts/evidence_test_helpers.sh

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start237-owner.txt"
BODY="$TMP_ROOT/himate-start237-body.json"
DOWNLOAD="$TMP_ROOT/himate-start237-evidence.pdf"
REPORT_PDF="$TMP_ROOT/himate-start237-report.pdf"
rm -f "$COOKIE" "$BODY" "$DOWNLOAD" "$REPORT_PDF"
trap 'rm -f "$COOKIE" "$BODY" "$DOWNLOAD" "$REPORT_PDF"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
TODAY="$(date -u +%Y-%m-%d)"
METRIC_KEY="ci.start237.$STAMP"
GROUP_KEY="ci_237_$STAMP"
MODULE_KEY="ci.start237_$STAMP"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-23.7 owner login... '
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'create partner plus module used by evidence/impact acceptance... '
partner_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({
 "display_name":"START 23.7 Evidence Partner "+s,
 "legal_name":"START 23.7 Evidence Partner LLC "+s,
 "brand_name":"START237 "+s,
 "contact_name":"Evidence Owner",
 "contact_email":"start237-"+s+"@example.com",
 "country":"US"
}))
PY
)"
partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
test -n "$partner_id"
group_payload="$(python3 - "$GROUP_KEY" "$STAMP" <<'PY'
import json,sys
print(json.dumps({"group_key":sys.argv[1],"label_en":"Evidence & Impact "+sys.argv[2],"label_hu":"Bizonyíték és hatás "+sys.argv[2],"sort_order":237}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$group_payload" "$BASE_URL/api/v1/module-groups" >/dev/null
module_payload="$(python3 - "$MODULE_KEY" "$GROUP_KEY" "$STAMP" <<'PY'
import json,sys
key,group,s=sys.argv[1:]
print(json.dumps({
 "key":key,"group_key":group,
 "label_en":"Evidence Module "+s,"label_hu":"Bizonyíték Modul "+s,
 "description_en":"START-23.7 evidence acceptance","description_hu":"START-23.7 bizonyíték elfogadás",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0",
 "default_monthly_price":0,"default_activation_fee":0,
 "availability":"ACTIVE","module_type":"FEATURE","owner_team":"Platform",
 "manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$module_payload" "$BASE_URL/api/v1/modules" >/dev/null
echo ok

printf 'create bilingual metric definition and map it to module... '
definition_payload="$(python3 - "$METRIC_KEY" "$STAMP" <<'PY'
import json,sys
key,s=sys.argv[1:]
print(json.dumps({
 "metric_key":key,
 "label_en":"Verified Attendance "+s,
 "label_hu":"Ellenőrzött látogatottság "+s,
 "description_en":"START-23.7 metric definition",
 "description_hu":"START-23.7 mérőszám definíció",
 "unit":"count","aggregation":"SUM","scope":"PARTNER"
}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$definition_payload" "$BASE_URL/api/v1/impact/definitions" >/dev/null
map_payload="$(python3 - "$METRIC_KEY" <<'PY'
import json,sys
print(json.dumps({"items":[{"metric_key":sys.argv[1],"label":"Verified attendance"}]}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$map_payload" "$BASE_URL/api/v1/modules/$MODULE_KEY/impact-metrics" >/dev/null
mapping="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/modules/$MODULE_KEY/impact-metrics")"
printf '%s' "$mapping" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; x=next(i for i in d["items"] if i["metric_key"]==key); assert x["label"]=="Verified attendance"' "$METRIC_KEY"
definitions="$(curl -fsS -b "$COOKIE" -H 'X-Himate-Locale: hu_HU' "$BASE_URL/api/v1/impact/definitions")"
printf '%s' "$definitions" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; x=next(i for i in d["items"] if i["metric_key"]==key); assert x["label_hu"].startswith("Ellenőrzött"),x' "$METRIC_KEY"
echo ok

printf 'record manual impact value and baseline with summary readback... '
manual_payload="$(python3 - "$partner_id" "$METRIC_KEY" "$TODAY" <<'PY'
import json,sys
partner,key,day=sys.argv[1:]
print(json.dumps({"partner_id":partner,"metric_key":key,"period_start":day,"period_end":day,"numeric_value":10,"provenance":"MANUAL","source_ref":"start-23.7-manual"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$manual_payload" "$BASE_URL/api/v1/impact/values" >/dev/null
baseline_payload="$(python3 - "$partner_id" "$METRIC_KEY" "$TODAY" <<'PY'
import json,sys
partner,key,day=sys.argv[1:]
print(json.dumps({"partner_id":partner,"metric_key":key,"period_start":day,"period_end":day,"numeric_value":4,"provenance":"MANUAL","source_ref":"start-23.7-baseline"}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$baseline_payload" "$BASE_URL/api/v1/impact/baselines" >/dev/null
summary="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/impact/summary?partner_id=$partner_id")"
printf '%s' "$summary" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; x=next(i for i in d["items"] if i["metric_key"]==key); assert float(x["numeric_value"])==10; assert float(x["baseline_numeric_value"])==4; assert float(x["delta_from_baseline"])==6' "$METRIC_KEY"
echo ok

printf 'create URL and declaration Evidence metadata records... '
url_payload="$(python3 - "$partner_id" "$METRIC_KEY" "$TODAY" <<'PY'
import json,sys
partner,key,day=sys.argv[1:]
print(json.dumps({"partner_id":partner,"metric_key":key,"evidence_type":"URL","title":"START 23.7 URL Evidence","description":"Metadata-only URL evidence","period_start":day,"period_end":day,"source_url":"https://example.com/start-23-7"}))
PY
)"
url_evidence="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$url_payload" "$BASE_URL/api/v1/evidence")"
printf '%s' "$url_evidence" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["evidence_type"]=="URL"; assert d["has_file"] is False; assert d["source_url"].startswith("https://")'
declaration_payload="$(python3 - "$partner_id" "$TODAY" <<'PY'
import json,sys
partner,day=sys.argv[1:]
print(json.dumps({"partner_id":partner,"metric_key":"","evidence_type":"PARTNER_DECLARATION","title":"START 23.7 Declaration","description":"Declaration evidence","period_start":day,"period_end":day,"declaration_text":"Partner confirms this acceptance evidence record."}))
PY
)"
declaration="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$declaration_payload" "$BASE_URL/api/v1/evidence")"
printf '%s' "$declaration" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["evidence_type"]=="PARTNER_DECLARATION"; assert d["has_file"] is False; assert d["declaration_text"]'
echo ok

printf 'upload real PDF Evidence and prove preview/download/SHA-256 integrity... '
evidence_id="$(create_pdf_evidence "$COOKIE" "$partner_id" "PDF" "START 23.7 Verified PDF" "start237-metric" "$METRIC_KEY")"
evidence="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/evidence/$evidence_id")"
expected_sha="$(printf '%s' "$evidence" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["has_file"] is True; assert d["mime_type"]=="application/pdf"; print(d["sha256"])')"
preview_headers="$(curl -fsS -D - -o /dev/null -b "$COOKIE" "$BASE_URL/api/v1/evidence/$evidence_id/preview")"
printf '%s' "$preview_headers" | grep -qi 'Content-Disposition: inline'
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/evidence/$evidence_id/download" -o "$DOWNLOAD"
actual_sha="$(python3 - "$DOWNLOAD" <<'PY'
import hashlib,sys
print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())
PY
)"
test "$actual_sha" = "$expected_sha"
python3 - "$DOWNLOAD" <<'PY'
import sys
raw=open(sys.argv[1],"rb").read()
assert raw.startswith(b"%PDF-"),raw[:16]
PY
echo ok

printf 'verify Evidence and create VERIFIED_DOCUMENT impact observation... '
verified="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"verification_status":"VERIFIED"}' "$BASE_URL/api/v1/evidence/$evidence_id")"
printf '%s' "$verified" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["verification_status"]=="VERIFIED"; assert d["verified_by"]'
verified_payload="$(python3 - "$partner_id" "$METRIC_KEY" "$TODAY" "$evidence_id" <<'PY'
import json,sys
partner,key,day,evidence=sys.argv[1:]
print(json.dumps({"partner_id":partner,"metric_key":key,"period_start":day,"period_end":day,"numeric_value":6,"provenance":"VERIFIED_DOCUMENT","evidence_id":evidence}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$verified_payload" "$BASE_URL/api/v1/impact/values" >/dev/null
values="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/impact/values?partner_id=$partner_id&metric_key=$METRIC_KEY")"
printf '%s' "$values" | python3 -c 'import json,sys; d=json.load(sys.stdin); evidence=sys.argv[1]; x=next(i for i in d["items"] if i["provenance"]=="VERIFIED_DOCUMENT"); assert x["evidence_id"]==evidence; assert x["source_ref"]==evidence; assert float(x["numeric_value"])==6' "$evidence_id"
echo ok

printf 'Billing rejects fake commercial references and accepts integrity-backed Evidence only... '
fake_payload='{"kind":"INVOICE","name":"Fake invoice","storage_url":"evidence://does-not-exist","note":"must fail"}'
test "$(status "$COOKIE" POST "/api/v1/billing/partners/$partner_id/documents" -H 'Content-Type: application/json' -d "$fake_payload")" = "409"
grep -q 'EVIDENCE_REFERENCE_INVALID' "$BODY"
invoice_evidence_id="$(create_pdf_evidence "$COOKIE" "$partner_id" "INVOICE" "START 23.7 Activation Invoice" "start237-invoice")"
billing_payload="$(python3 - "$invoice_evidence_id" <<'PY'
import json,sys
print(json.dumps({"kind":"INVOICE","name":"START 23.7 Activation Invoice","storage_url":"evidence://"+sys.argv[1],"note":"Real Evidence-backed invoice"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$billing_payload" "$BASE_URL/api/v1/billing/partners/$partner_id/documents" >/dev/null
documents="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/documents")"
printf '%s' "$documents" | python3 -c 'import json,sys; d=json.load(sys.stdin); evidence=sys.argv[1]; x=next(i for i in d["items"] if i["storage_url"]=="evidence://"+evidence); assert x["kind"]=="INVOICE"; assert x["mime_type"]=="application/pdf"; assert x["sha256"]; assert int(x["size_bytes"])>0' "$invoice_evidence_id"
echo ok

printf 'generate immutable report snapshot and download real PDF... '
report_payload="$(python3 - "$partner_id" "$TODAY" "$STAMP" <<'PY'
import json,sys
partner,day,s=sys.argv[1:]
print(json.dumps({"report_type":"PARTNER_IMPACT","title":"START 23.7 Evidence Report "+s,"partner_ids":[partner],"period_start":day,"period_end":day}))
PY
)"
created="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$report_payload" "$BASE_URL/api/v1/reports")"
report_id="$(printf '%s' "$created" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"] in ("QUEUED","RUNNING","READY"); print(d["id"])')"
report=""
i=0
while [ "$i" -lt 40 ]; do
  report="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/reports/$report_id")"
  report_status="$(printf '%s' "$report" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')"
  if [ "$report_status" = "READY" ]; then break; fi
  if [ "$report_status" = "FAILED" ]; then printf '%s
' "$report" >&2; exit 1; fi
  i=$((i+1))
  sleep 1
done
test "$report_status" = "READY"
snapshot_sha="$(printf '%s' "$report" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["download_ready"] is True; assert d["snapshot_sha256"]; assert d["pdf_sha256"]; assert int(d["pdf_size_bytes"])>0; assert len(d["evidence_ids"])>=2; print(d["snapshot_sha256"])')"
pdf_sha="$(printf '%s' "$report" | python3 -c 'import json,sys; print(json.load(sys.stdin)["pdf_sha256"])')"
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/reports/$report_id/download" -o "$REPORT_PDF"
python3 - "$REPORT_PDF" "$pdf_sha" <<'PY'
import hashlib,sys
raw=open(sys.argv[1],"rb").read()
assert raw.startswith(b"%PDF-"),raw[:16]
assert hashlib.sha256(raw).hexdigest()==sys.argv[2]
PY
echo ok

printf 'regenerate report from stored snapshot without rereading live state... '
# Mutate live Impact after the snapshot; regeneration must preserve the stored snapshot and PDF.
later_payload="$(python3 - "$partner_id" "$METRIC_KEY" "$TODAY" <<'PY'
import json,sys
partner,key,day=sys.argv[1:]
print(json.dumps({"partner_id":partner,"metric_key":key,"period_start":day,"period_end":day,"numeric_value":99,"provenance":"MANUAL","source_ref":"post-snapshot-live-change"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$later_payload" "$BASE_URL/api/v1/impact/values" >/dev/null
regenerated="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/api/v1/reports/$report_id/regenerate")"
printf '%s' "$regenerated" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="READY"; assert d["snapshot_sha256"]==sys.argv[1]; assert d["pdf_sha256"]==sys.argv[2]' "$snapshot_sha" "$pdf_sha"
echo ok

printf 'Evidence report linkage and central audit are visible... '
linked="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/evidence/$evidence_id")"
printf '%s' "$linked" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert sys.argv[1] in d["report_ids"],d' "$report_id"
audit="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/audit/events?limit=200")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); actions={x["action"] for x in d["items"]}; required={"EVIDENCE_VERIFICATION_CHANGED"}; assert required<=actions,(required-actions,actions)'
echo ok

echo "HIMATE START-23.7 Evidence, Impact & Reporting smoke passed"
