#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-223-owner.txt"
BODY="$TMP_ROOT/himate-223-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

TODAY="$(python3 - <<'PY'
from datetime import datetime,timezone
print(datetime.now(timezone.utc).date().isoformat())
PY
)"
PREV30="$(python3 - "$TODAY" <<'PY'
from datetime import date,timedelta
import sys
print((date.fromisoformat(sys.argv[1])-timedelta(days=30)).isoformat())
PY
)"
NEXT30="$(python3 - "$TODAY" <<'PY'
from datetime import date,timedelta
import sys
print((date.fromisoformat(sys.argv[1])+timedelta(days=30)).isoformat())
PY
)"
NEXT60="$(python3 - "$TODAY" <<'PY'
from datetime import date,timedelta
import sys
print((date.fromisoformat(sys.argv[1])+timedelta(days=60)).isoformat())
PY
)"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-22.3 system owner login... '
payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'create START-22.3 commercial partner and module... '
partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json'   -d '{"display_name":"START 22.3 Commercial Partner","legal_name":"START 22.3 Commercial Partner LLC","brand_name":"START 22.3","contact_name":"Commercial Owner","contact_email":"start223-commercial@example.com","country":"US","primary_domain":"start223.example.com","website":"https://start223.example.com"}'   "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
test -n "$partner_id"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json'   -d '{"group_key":"ci_223","label":"CI START 22.3","sort_order":93}'   "$BASE_URL/api/v1/module-groups" >/dev/null
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json'   -d '{"key":"ci.commercial_snapshot","label":"Commercial Snapshot Module","group_key":"ci_223","description":"START-22.3 immutable period pricing","currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":50,"availability":"ACTIVE","module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}}'   "$BASE_URL/api/v1/modules" >/dev/null
echo "$partner_id"

printf 'commercial workflow fails closed before agreement/evidence/payment... '
terms_payload="$(python3 - "$PREV30" <<'PY'
import json,sys
print(json.dumps({
 "currency":"USD","activation_fee":13000,"activation_fee_waived":False,"activation_fee_reason":"",
 "base_monthly_fee":100,"annual_increase_percent":0,"price_effective_from":sys.argv[1],
 "service_anchor_date":sys.argv[1],"reason":"START-22.3 acceptance commercial terms"
}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$terms_payload" "$BASE_URL/api/v1/billing/partners/$partner_id/terms" >/dev/null
blocked="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/commercial-status")"
printf '%s' "$blocked" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["provisioning_allowed"] is False; assert d["next_action"]=="CONFIRM_COMMERCIAL_AGREEMENT"'
echo ok

printf 'agreement and activation invoice are recorded explicitly... '
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json'   -d '{"status":"AGREED","agreement_reference":"contract://start223/signed-001","note":"START-22.3 acceptance agreement"}'   "$BASE_URL/api/v1/billing/partners/$partner_id/agreement" >/dev/null
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json'   -d '{"kind":"INVOICE","name":"Activation Fee Invoice","storage_url":"evidence://start223/activation-invoice-001","note":"START-22.3 activation invoice","mime_type":"application/pdf","sha256":"","size_bytes":0}'   "$BASE_URL/api/v1/billing/partners/$partner_id/documents" >/dev/null
no_payment_evidence="$(status "$COOKIE" PUT "/api/v1/billing/partners/$partner_id/license" -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":13000,"paid_amount":13000,"payment_date":"'"$TODAY"'","payment_reference":"PAY-START223-001","verified_by":"start223-ci","note":"Should fail before payment evidence","waived":false,"waiver_reason":""}')"
test "$no_payment_evidence" = "409"
grep -q 'PAYMENT_EVIDENCE_REQUIRED' "$BODY"
echo ok

printf 'payment evidence unlocks PAID license and provisioning gate... '
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json'   -d '{"kind":"PAYMENT_EVIDENCE","name":"Activation payment receipt","storage_url":"evidence://start223/payment-001","note":"Verified bank receipt","mime_type":"application/pdf","sha256":"","size_bytes":0}'   "$BASE_URL/api/v1/billing/partners/$partner_id/documents" >/dev/null
license_payload="$(python3 - "$TODAY" <<'PY'
import json,sys
print(json.dumps({"currency":"USD","required_amount":13000,"paid_amount":13000,"payment_date":sys.argv[1],"payment_reference":"PAY-START223-001","verified_by":"start223-ci","note":"Verified START-22.3 activation payment","waived":False,"waiver_reason":""}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$license_payload" "$BASE_URL/api/v1/billing/partners/$partner_id/license" >/dev/null
ready="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/commercial-status")"
printf '%s' "$ready" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["provisioning_allowed"] is True; assert d["payment"]["status"]=="PAID"; assert d["evidence"]["invoice_count"]>=1; assert d["evidence"]["payment_evidence_count"]>=1; assert all(x["complete"] is True for x in d["workflow"])'
echo ok

printf 'module activation backfills 30-day snapshots from authoritative activation date... '
module_payload="$(python3 - "$PREV30" <<'PY'
import json,sys
print(json.dumps({"status":"ACTIVE","visible":True,"included_in_base":False,"partner_price":50,"price_effective_at":sys.argv[1],"reason":"START-22.3 initial module price"}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$module_payload" "$BASE_URL/api/v1/partners/$partner_id/modules/ci.commercial_snapshot" >/dev/null
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1   -c "UPDATE catalog.partner_modules SET activated_at='$PREV30'::date WHERE partner_id='$partner_id' AND module_key='ci.commercial_snapshot';" >/dev/null
summary="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/summary")"
printf '%s' "$summary" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["cycle_days"]==30; assert d["extra_module_fee"]==50,d'
snapshot_count="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.module_period_snapshots WHERE partner_id='$partner_id' AND module_key='ci.commercial_snapshot'")"
test "$snapshot_count" -ge "2"
past_price="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT price_snapshot FROM billing.module_period_snapshots WHERE partner_id='$partner_id' AND module_key='ci.commercial_snapshot' AND period_start='$PREV30'::date")"
current_price="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT price_snapshot FROM billing.module_period_snapshots WHERE partner_id='$partner_id' AND module_key='ci.commercial_snapshot' AND period_start='$TODAY'::date")"
test "$past_price" = "50.00"
test "$current_price" = "50.00"
echo ok

printf 'current period price is immutable after a catalog price change... '
price_change="$(python3 - "$TODAY" <<'PY'
import json,sys
print(json.dumps({"partner_price":90,"price_effective_at":sys.argv[1],"reason":"START-22.3 next-period price"}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$price_change" "$BASE_URL/api/v1/partners/$partner_id/modules/ci.commercial_snapshot" >/dev/null
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/summary" >/dev/null
sub_now="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions")"
printf '%s' "$sub_now" | python3 -c 'import json,sys; d=json.load(sys.stdin); s=next(x for x in d["items"] if x["module_key"]=="ci.commercial_snapshot"); assert s["price"]==50,s'
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT price_snapshot FROM billing.module_period_snapshots WHERE partner_id='$partner_id' AND module_key='ci.commercial_snapshot' AND period_start='$TODAY'::date")" = "50.00"
echo ok

printf 'invoice cycle uses immutable item lines, never the live catalog price... '
docker compose exec -T billing /app/service --run-invoice-cycle "$TODAY"
invoice_today="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/invoices")"
printf '%s' "$invoice_today" | python3 -c 'import json,sys; d=json.load(sys.stdin); inv=next(x for x in d["items"] if str(x["service_period_end_exclusive"])[:10]==sys.argv[1]); mods=[i for i in inv["items"] if i["item_type"]=="MODULE" and i["module_key"]=="ci.commercial_snapshot"]; assert len(mods)==1,inv; assert mods[0]["amount"]==50,mods; assert inv["base_fee"]==100 and inv["module_fee"]==50 and inv["total"]==150,inv' "$TODAY"
echo ok

printf 'next module period adopts the new price snapshot... '
docker compose exec -T billing /app/service --run-invoice-cycle "$NEXT30"
sub_next="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions")"
printf '%s' "$sub_next" | python3 -c 'import json,sys; d=json.load(sys.stdin); s=next(x for x in d["items"] if x["module_key"]=="ci.commercial_snapshot"); assert s["price"]==90,s; assert str(s["period_start"])[:10]==sys.argv[1],s' "$NEXT30"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT price_snapshot FROM billing.module_period_snapshots WHERE partner_id='$partner_id' AND module_key='ci.commercial_snapshot' AND period_start='$NEXT30'::date")" = "90.00"
invoice_next="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/invoices")"
printf '%s' "$invoice_next" | python3 -c 'import json,sys; d=json.load(sys.stdin); inv=next(x for x in d["items"] if str(x["service_period_end_exclusive"])[:10]==sys.argv[1]); mods=[i for i in inv["items"] if i["item_type"]=="MODULE" and i["module_key"]=="ci.commercial_snapshot"]; assert len(mods)==1,inv; assert mods[0]["amount"]==50,mods' "$NEXT30"
echo ok

printf 'period-end cancellation emits an immutable billing event... '
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"cancel_at_period_end":true,"reason":"START-22.3 acceptance cancellation"}'   "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions/ci.commercial_snapshot" >/dev/null
events="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/events")"
printf '%s' "$events" | python3 -c 'import json,sys; d=json.load(sys.stdin); types={x["event_type"] for x in d["items"]}; required={"COMMERCIAL_AGREEMENT_CONFIRMED","ACTIVATION_INVOICE_REGISTERED","PAYMENT_EVIDENCE_REGISTERED","LICENSE_PAID","MODULE_ACTIVATED","MODULE_PERIOD_STARTED","MODULE_RENEWED","INVOICE_ITEM_CREATED","INVOICE_GENERATED","MODULE_CANCELLATION_SCHEDULED"}; missing=required-types; assert not missing,missing'
echo ok

printf 'website adapter fails closed on authoritative domain mismatch... '
bad_adapter='{"environment":"PRODUCTION","adapter_type":"GENERIC_HTTP","site_base_url":"https://wrong.example.net","allowed_domains":["wrong.example.net"],"capabilities":["ENTITLEMENTS","HEARTBEAT","METRICS","AGGREGATED_DATA","RECONCILIATION"],"enabled":true,"config":{}}'
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$bad_adapter" "$BASE_URL/api/v1/connectors/$partner_id/website-adapter" >/dev/null
credential="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"environment":"PRODUCTION"}' "$BASE_URL/api/v1/connectors/$partner_id/credential")"
connector_token="$(printf '%s' "$credential" | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')"
bad_state_code="$(curl -sS -o "$BODY" -w '%{http_code}' -H "Authorization: Bearer $connector_token" "$BASE_URL/connector/v1/commercial-state")"
test "$bad_state_code" = "409"
grep -q 'DOMAIN_BINDING_MISMATCH' "$BODY"
echo ok

printf 'correct adapter exposes minimized credential-bound commercial state... '
good_adapter='{"environment":"PRODUCTION","adapter_type":"GENERIC_HTTP","site_base_url":"https://start223.example.com","allowed_domains":["start223.example.com"],"capabilities":["ENTITLEMENTS","HEARTBEAT","METRICS","AGGREGATED_DATA","RECONCILIATION"],"enabled":true,"config":{"partner_portal":"/partner/login"}}'
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$good_adapter" "$BASE_URL/api/v1/connectors/$partner_id/website-adapter" >/dev/null
commercial_state="$(curl -fsS -H "Authorization: Bearer $connector_token" "$BASE_URL/connector/v1/commercial-state")"
printf '%s' "$commercial_state" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]; assert d["tenant_scope"]=="CREDENTIAL_BOUND"; assert d["contract_version"]=="START-22.3"; assert d["domain_binding"]["primary_domain"]=="start223.example.com"; assert "ci.commercial_snapshot" in d["entitlements"]; assert set(d["entitlements"]["ci.commercial_snapshot"].keys())=={"status","version","included_in_base"}; assert "price" not in d and "billing" not in d and "invoices" not in d and "payments" not in d; dc=d["data_contract"]; assert dc["privacy_mode"]=="AGGREGATED_ONLY"; assert dc["raw_invoice_data_allowed"] is False and dc["raw_payment_data_allowed"] is False; assert dc["retention_policy"]=="7_YEARS"' "$partner_id"
echo ok

printf 'website adapter configuration is visible to the HIMATE control plane... '
adapter="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/connectors/$partner_id/website-adapter?environment=PRODUCTION")"
printf '%s' "$adapter" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["configured"] is True and d["enabled"] is True; assert "start223.example.com" in d["allowed_domains"]; assert d["privacy_mode"]=="AGGREGATED_ONLY"'
echo ok

echo "HIMATE START-22.3 Commercial Automation & Partner Website Integration smoke passed"
