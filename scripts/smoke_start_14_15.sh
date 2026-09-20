#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE_JAR="${TMPDIR:-/tmp}/himate-start-14-15-cookies.txt"
BODY="${TMPDIR:-/tmp}/himate-start-14-15-body.json"
PDF="${TMPDIR:-/tmp}/himate-start-14-15-evidence.pdf"
REPORT_PDF="${TMPDIR:-/tmp}/himate-start-14-15-report.pdf"
rm -f "$COOKIE_JAR" "$BODY" "$PDF" "$REPORT_PDF"
trap 'rm -f "$COOKIE_JAR" "$BODY" "$PDF" "$REPORT_PDF"' EXIT

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

printf 'login... '
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"email":"admin@example.com","password":"local-development-password"}'   "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'START-14-15 services are healthy... '
health="$(curl -fsS "$BASE_URL/api/v1/health")"
printf '%s' "$health" | grep -q '"evidence":"ok"'
printf '%s' "$health" | grep -q '"reports":"ok"'
echo ok

printf 'create evidence test partners... '
p1="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"display_name":"Evidence Partner A","legal_name":"Evidence Partner A LLC","category_id":"cat_006","lifecycle":"PROSPECT","contact_name":"Evidence Admin","contact_email":"evidence-a@example.com","country":"United States"}'   "$BASE_URL/api/v1/partners")"
partner1="$(printf '%s' "$p1" | json_field id)"
p2="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"display_name":"Evidence Partner B","legal_name":"Evidence Partner B LLC","category_id":"cat_006","lifecycle":"PROSPECT","contact_name":"Evidence Admin","contact_email":"evidence-b@example.com","country":"United States"}'   "$BASE_URL/api/v1/partners")"
partner2="$(printf '%s' "$p2" | json_field id)"
test -n "$partner1"
test -n "$partner2"
echo ok

printf 'create START-14 metric... '
metric_key="ci.evidence"
metric_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"metric_key":"ci.evidence","label":"Evidence CI","description":"START-14 evidence metric","unit":"count","aggregation":"SUM","scope":"PARTNER"}'   "$BASE_URL/api/v1/impact/definitions")"
test "$metric_code" = "201" || test "$metric_code" = "409"
echo ok

printf 'reject fake PDF content... '
printf '<html>not a pdf</html>' > "$PDF"
bad_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR"   -F "partner_id=$partner1" -F "metric_key=$metric_key" -F "evidence_type=PDF" -F "title=Bad PDF"   -F "period_start=2026-09-01" -F "period_end=2026-09-20" -F "file=@$PDF;type=application/pdf"   "$BASE_URL/api/v1/evidence")"
test "$bad_code" = "415"
echo ok

printf 'upload checksum-backed PDF evidence... '
printf '%%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%%%EOF\n' > "$PDF"
evidence="$(curl -fsS -b "$COOKIE_JAR"   -F "partner_id=$partner1" -F "metric_key=$metric_key" -F "evidence_type=PDF" -F "title=Verified CI Evidence"   -F "description=Evidence smoke document" -F "period_start=2026-09-01" -F "period_end=2026-09-20"   -F "file=@$PDF;type=application/pdf"   "$BASE_URL/api/v1/evidence")"
evidence_id="$(printf '%s' "$evidence" | json_field id)"
test -n "$evidence_id"
printf '%s' "$evidence" | python3 -c 'import json,sys,re; d=json.load(sys.stdin); assert d["mime_type"]=="application/pdf"; assert d["size_bytes"]>0; assert re.fullmatch(r"[0-9a-f]{64}",d["sha256"])'
integrity="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/evidence/$evidence_id/integrity")"
printf '%s' "$integrity" | grep -q '"valid":true'
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/evidence/$evidence_id/preview" -o "$BODY"
cmp "$PDF" "$BODY"
echo ok

printf 'verify evidence... '
curl -fsS -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json'   -d '{"verification_status":"VERIFIED"}'   "$BASE_URL/api/v1/evidence/$evidence_id" | grep -q '"verification_status":"VERIFIED"'
echo ok

printf 'VERIFIED_DOCUMENT requires a real verified evidence record... '
verified="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner1\",\"metric_key\":\"$metric_key\",\"period_start\":\"2026-09-01\",\"period_end\":\"2026-09-20\",\"numeric_value\":9,\"provenance\":\"VERIFIED_DOCUMENT\",\"evidence_id\":\"$evidence_id\"}"   "$BASE_URL/api/v1/impact/values")"
printf '%s' "$verified" | grep -q '"provenance":"VERIFIED_DOCUMENT"'
curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d "{\"partner_id\":\"$partner1\",\"metric_key\":\"$metric_key\",\"period_start\":\"2025-01-01\",\"period_end\":\"2025-01-31\",\"numeric_value\":100,\"provenance\":\"MANUAL\",\"source_ref\":\"outside-report-period\"}" \
  "$BASE_URL/api/v1/impact/values" >/dev/null
period_summary="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/impact/summary?partner_id=$partner1&period_start=2026-09-01&period_end=2026-09-20")"
printf '%s' "$period_summary" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=next(x for x in d["items"] if x["metric_key"]=="ci.evidence"); assert m["numeric_value"]==9, m'
fake_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner1\",\"metric_key\":\"$metric_key\",\"period_start\":\"2026-09-01\",\"period_end\":\"2026-09-20\",\"numeric_value\":10,\"provenance\":\"VERIFIED_DOCUMENT\",\"evidence_id\":\"evd_missing\"}"   "$BASE_URL/api/v1/impact/values")"
test "$fake_code" = "409"
tenant_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner2\",\"metric_key\":\"$metric_key\",\"period_start\":\"2026-09-01\",\"period_end\":\"2026-09-20\",\"numeric_value\":10,\"provenance\":\"VERIFIED_DOCUMENT\",\"evidence_id\":\"$evidence_id\"}"   "$BASE_URL/api/v1/impact/values")"
test "$tenant_code" = "409"
echo ok

printf 'URL and partner declaration evidence... '
url_ev="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner1\",\"metric_key\":\"$metric_key\",\"evidence_type\":\"URL\",\"title\":\"Source URL\",\"period_start\":\"2026-09-01\",\"period_end\":\"2026-09-20\",\"source_url\":\"https://example.org/evidence\"}"   "$BASE_URL/api/v1/evidence")"
printf '%s' "$url_ev" | grep -q '"evidence_type":"URL"'
url_evidence_id="$(printf '%s' "$url_ev" | json_field id)"
curl -fsS -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json' \
  -d '{"verification_status":"VERIFIED"}' \
  "$BASE_URL/api/v1/evidence/$url_evidence_id" >/dev/null
url_verified_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d "{\"partner_id\":\"$partner1\",\"metric_key\":\"$metric_key\",\"period_start\":\"2026-09-01\",\"period_end\":\"2026-09-20\",\"numeric_value\":11,\"provenance\":\"VERIFIED_DOCUMENT\",\"evidence_id\":\"$url_evidence_id\"}" \
  "$BASE_URL/api/v1/impact/values")"
test "$url_verified_code" = "409"
decl_ev="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner1\",\"metric_key\":\"$metric_key\",\"evidence_type\":\"PARTNER_DECLARATION\",\"title\":\"Partner declaration\",\"period_start\":\"2026-09-01\",\"period_end\":\"2026-09-20\",\"declaration_text\":\"Partner confirms the reported result for the period.\"}"   "$BASE_URL/api/v1/evidence")"
printf '%s' "$decl_ev" | grep -q '"evidence_type":"PARTNER_DECLARATION"'
bad_url_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"partner_id\":\"$partner1\",\"evidence_type\":\"URL\",\"title\":\"Bad URL\",\"source_url\":\"file:///etc/passwd\"}"   "$BASE_URL/api/v1/evidence")"
test "$bad_url_code" = "400"
paged="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/evidence?partner_id=$partner1&limit=1&offset=0")"
printf '%s' "$paged" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["count"]==1; assert d["total"]>=3; assert d["has_more"] is True'
searched="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/evidence?q=Verified%20CI&limit=10&offset=0")"
printf '%s' "$searched" | grep -q "$evidence_id"
outside="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/evidence?partner_id=$partner1&period_start=2025-01-01&period_end=2025-01-31")"
printf '%s' "$outside" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["total"]==0, d'
echo ok

printf 'evidence file round-trip... '
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/evidence/$evidence_id/download" -o "$BODY"
cmp "$PDF" "$BODY"
echo ok

wait_report() {
  id="$1"
  for i in $(seq 1 40); do
    item="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/reports/$id")"
    status="$(printf '%s' "$item" | json_field status)"
    if [ "$status" = "READY" ]; then
      printf '%s' "$item"
      return 0
    fi
    if [ "$status" = "FAILED" ]; then
      printf '%s\n' "$item" >&2
      return 1
    fi
    sleep 1
  done
  return 1
}

printf 'START-15 partner PDF report background job... '
report="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"report_type\":\"PARTNER_IMPACT\",\"title\":\"CI Partner Impact Report\",\"partner_ids\":[\"$partner1\"],\"period_start\":\"2026-09-01\",\"period_end\":\"2026-09-20\"}"   "$BASE_URL/api/v1/reports")"
report_id="$(printf '%s' "$report" | json_field id)"
ready="$(wait_report "$report_id")"
printf '%s' "$ready" | grep -q '"status":"READY"'
printf '%s' "$ready" | grep -q "$evidence_id"
printf '%s' "$ready" | grep -q '"template_version":"impact-v1"'
printf '%s' "$ready" | python3 -c 'import json,sys,re; d=json.load(sys.stdin); assert re.fullmatch(r"[0-9a-f]{64}",d["snapshot_sha256"]); sections=d["snapshot"]["metric_sections"]; metric=next(m for s in sections for m in s["metrics"] if m["metric_key"]=="ci.evidence"); assert metric["numeric_value"]==9, metric'
report_hash="$(printf '%s' "$ready" | json_field pdf_sha256)"
snapshot_hash="$(printf '%s' "$ready" | json_field snapshot_sha256)"
test -n "$report_hash"
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/reports/$report_id/download" -o "$REPORT_PDF"
head -c 8 "$REPORT_PDF" | grep -q '%PDF-1.4'
grep -a -q "$report_id" "$REPORT_PDF"
grep -a -q 'Template: impact-v1' "$REPORT_PDF"
grep -a -q "Snapshot: $snapshot_hash" "$REPORT_PDF"
echo ok

printf 'report PDF is reproducible from stored snapshot... '
regen="$(curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/reports/$report_id/regenerate")"
regen_hash="$(printf '%s' "$regen" | json_field pdf_sha256)"
test "$regen_hash" = "$report_hash"
echo ok

printf 'Evidence Library links evidence to report... '
linked="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/evidence?report_id=$report_id")"
printf '%s' "$linked" | grep -q "$evidence_id"
echo ok

printf 'multi-partner and global report types... '
multi="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d "{\"report_type\":\"MULTI_PARTNER\",\"partner_ids\":[\"$partner1\",\"$partner2\"],\"period_start\":\"2026-09-01\",\"period_end\":\"2026-09-20\"}"   "$BASE_URL/api/v1/reports")"
multi_id="$(printf '%s' "$multi" | json_field id)"
wait_report "$multi_id" >/dev/null
global="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"report_type":"HIMATE_GLOBAL","period_start":"2026-09-01","period_end":"2026-09-20"}'   "$BASE_URL/api/v1/reports")"
global_id="$(printf '%s' "$global" | json_field id)"
wait_report "$global_id" >/dev/null
echo ok

echo "HIMATE START-14–15 integration smoke passed"
