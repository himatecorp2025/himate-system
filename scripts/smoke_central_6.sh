#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-central6-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-central6-partner.txt"
BODY="$TMP_ROOT/himate-central6-body.json"
PDF="$TMP_ROOT/himate-central6-invoice.pdf"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY" "$PDF"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY" "$PDF"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
PAID_EMAIL="central6.paid.$STAMP@himate.test"
PAID_PASSWORD="Central6Paid!2345Aa"
PAID_REQUEST="central6-paid-$STAMP"
ZERO_REQUEST="central6-zero-$STAMP"

owner_login="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$owner_login" "$BASE_URL/api/v1/auth/login" >/dev/null

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'registered paid partner enters Central-6 onboarding queue with Portal disabled... '
paid_payload="$(python3 - "$STAMP" "$PAID_EMAIL" "$PAID_REQUEST" <<'PY'
import json,sys
s,email,request=sys.argv[1:]
print(json.dumps({
  "display_name":"Central-6 Paid Partner "+s,
  "legal_name":"Central-6 Paid Partner "+s+" LLC",
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "contact_name":"Central-6 Paid Owner",
  "contact_email":email,
  "finance_contact_name":"Central-6 Finance",
  "finance_contact_email":email,
  "country":"United States",
  "onboarding_request_id":request
}))
PY
)"
paid_partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$paid_payload" "$BASE_URL/api/v1/partners")"
PAID_ID="$(printf '%s' "$paid_partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
paid_onboarding="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$PAID_ID/onboarding")"
printf '%s' "$paid_onboarding" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="REGISTERED" and d["classification"]=="UNCLASSIFIED" and d["portal_enabled"] is False,d'
echo ok

printf 'Portal identity may exist, but authentication is blocked before final approval... '
portal_payload="$(python3 - "$PAID_EMAIL" "$PAID_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Central-6 Paid Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_payload" "$BASE_URL/api/v1/partners/$PAID_ID/portal-users" >/dev/null
partner_login="$(python3 - "$PAID_EMAIL" "$PAID_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
blocked="$(curl -sS -o "$BODY" -w '%{http_code}' -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_login" "$BASE_URL/partner/api/v1/auth/login")"
test "$blocked" = "403"
grep -q 'PARTNER_ACCESS_DISABLED' "$BODY"
rm -f "$PARTNER_COOKIE"
echo ok

printf 'HIMATE review and PAID classification advance the controlled onboarding state machine... '
reviewed="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"state":"PENDING_REVIEW","reason":"Central-6 acceptance review"}' "$BASE_URL/api/v1/billing/partners/$PAID_ID/onboarding")"
printf '%s' "$reviewed" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="PENDING_REVIEW",d'
classified="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"state":"CLASSIFIED","classification":"PAID","reason":"Central-6 paid classification"}' "$BASE_URL/api/v1/billing/partners/$PAID_ID/onboarding")"
printf '%s' "$classified" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="CLASSIFIED" and d["classification"]=="PAID" and d["portal_enabled"] is False,d'
echo ok

printf 'manual invoice starts as DRAFT and moves onboarding to INVOICE_PENDING... '
invoice_payload="$(python3 - "$PAID_ID" <<'PY'
import json,sys
print(json.dumps({
  "partner_id":sys.argv[1],
  "currency":"USD",
  "description":"Central-6 onboarding and service activation",
  "net_amount":1234.56,
  "notes":"Central-6 paid onboarding acceptance"
}))
PY
)"
invoice="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$invoice_payload" "$BASE_URL/api/v1/billing/invoices")"
INVOICE_ID="$(printf '%s' "$invoice" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["workflow_status"]=="DRAFT",d; assert d["gross_total"]>=d["net_total"]>0,d; print(d["id"])')"
onboarding="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$PAID_ID/onboarding")"
printf '%s' "$onboarding" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="INVOICE_PENDING",d'
draft_pdf="$(status "$OWNER_COOKIE" GET "/api/v1/billing/invoices/$INVOICE_ID/pdf")"
test "$draft_pdf" = "409"
grep -q 'INVOICE_NOT_APPROVED' "$BODY"
echo ok

printf 'invoice requires explicit approval, distribution, then payment... '
approved="$(curl -fsS -b "$OWNER_COOKIE" -X POST -H 'Content-Type: application/json' -d '{"reason":"Central-6 finance approval"}' "$BASE_URL/api/v1/billing/invoices/$INVOICE_ID/approve")"
printf '%s' "$approved" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["workflow_status"]=="APPROVED" and d["approved_by"],d'
premature_paid="$(status "$OWNER_COOKIE" POST "/api/v1/billing/invoices/$INVOICE_ID/mark-paid" -H 'Content-Type: application/json' -d '{"reason":"Must be rejected before distribution","payment_reference":"C6-PREMATURE"}')"
test "$premature_paid" = "409"
grep -q 'INVOICE_STATE' "$BODY"
sent="$(curl -fsS -b "$OWNER_COOKIE" -X POST -H 'Content-Type: application/json' -d '{"reason":"Central-6 distribution approval"}' "$BASE_URL/api/v1/billing/invoices/$INVOICE_ID/send")"
printf '%s' "$sent" | python3 -c 'import datetime,json,sys; d=json.load(sys.stdin); assert d["workflow_status"]=="SENT" and d["sent_by"] and d["delivery_channel"]=="PORTAL_EMAIL",d; sent=datetime.datetime.fromisoformat(d["sent_at"].replace("Z","+00:00")); deadline=datetime.datetime.fromisoformat(d["payment_deadline_at"].replace("Z","+00:00")); assert abs((deadline-sent).total_seconds()-72*3600)<2,(sent,deadline)'
onboarding="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$PAID_ID/onboarding")"
printf '%s' "$onboarding" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="PAYMENT_PENDING" and d["portal_enabled"] is False,d'
paid="$(curl -fsS -b "$OWNER_COOKIE" -X POST -H 'Content-Type: application/json' -d '{"reason":"Central-6 manual finance settlement","payment_reference":"C6-MANUAL-PAID"}' "$BASE_URL/api/v1/billing/invoices/$INVOICE_ID/mark-paid")"
printf '%s' "$paid" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["workflow_status"]=="PAID" and d["status"]=="PAID" and d["provider"]=="MANUAL",d'
onboarding="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$PAID_ID/onboarding")"
printf '%s' "$onboarding" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="ADMIN_APPROVAL" and d["portal_enabled"] is False,d'
echo ok

printf 'final HIMATE approval activates Portal access and distributed invoice PDF... '
active="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"state":"ACTIVE","reason":"Central-6 final HIMATE approval"}' "$BASE_URL/api/v1/billing/partners/$PAID_ID/onboarding")"
printf '%s' "$active" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="ACTIVE" and d["portal_enabled"] is True,d'
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_login" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
portal_invoices="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/billing/invoices")"
printf '%s' "$portal_invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); iid=sys.argv[1]; x=next(v for v in d["items"] if v["id"]==iid); assert x["workflow_status"]=="PAID",x' "$INVOICE_ID"
curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/billing/invoices/$INVOICE_ID/pdf" -o "$PDF"
test "$(head -c 4 "$PDF")" = "%PDF"
echo ok

printf 'Sponsored zero-dollar onboarding is documented without generating an invoice... '
zero_payload="$(python3 - "$STAMP" "$ZERO_REQUEST" <<'PY'
import json,sys
s,request=sys.argv[1:]
print(json.dumps({
  "display_name":"Central-6 Sponsored Partner "+s,
  "legal_name":"Central-6 Sponsored Partner "+s+" Foundation",
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "contact_name":"Central-6 Sponsored Owner",
  "contact_email":"central6.zero."+s+"@himate.test",
  "country":"United States",
  "onboarding_request_id":request
}))
PY
)"
zero_partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$zero_payload" "$BASE_URL/api/v1/partners")"
ZERO_ID="$(printf '%s' "$zero_partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"state":"PENDING_REVIEW","reason":"Central-6 sponsored review"}' "$BASE_URL/api/v1/billing/partners/$ZERO_ID/onboarding" >/dev/null
zero_classified="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"state":"CLASSIFIED","classification":"SPONSORED","nominal_value":5000,"currency":"USD","evidence_reference":"C6-SPONSOR-ACCEPTANCE","reason":"Approved sponsored access for Central-6 acceptance"}' "$BASE_URL/api/v1/billing/partners/$ZERO_ID/onboarding")"
printf '%s' "$zero_classified" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="CLASSIFIED" and d["classification"]=="SPONSORED",d'
zero_state="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$ZERO_ID/onboarding")"
printf '%s' "$zero_state" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["support_waiver_documented"] is True,d'
zero_invoices="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/invoices?partner_id=$ZERO_ID")"
printf '%s' "$zero_invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["count"]==0,d'
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"state":"ADMIN_APPROVAL","reason":"Zero-dollar support documentation verified"}' "$BASE_URL/api/v1/billing/partners/$ZERO_ID/onboarding" >/dev/null
zero_active="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"state":"ACTIVE","reason":"Central-6 sponsored final approval"}' "$BASE_URL/api/v1/billing/partners/$ZERO_ID/onboarding")"
printf '%s' "$zero_active" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="ACTIVE" and d["portal_enabled"] is True,d'
echo ok

printf 'Finance overview and immutable audit ledgers reflect Central-6 workflow... '
overview="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/finance/overview")"
printf '%s' "$overview" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["source"]=="CENTRAL_6_FINANCE_LEDGER",d; assert any(x["currency"]=="USD" and x["paid"]>=1 for x in d["currencies"]),d; assert all("currency" in x for x in d["monthly_paid"]),d'
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.finance_transactions WHERE invoice_id='$INVOICE_ID' AND status IN ('DRAFT','APPROVED','SENT','PAID');")" -ge "4"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.partner_onboarding_history WHERE partner_id='$PAID_ID';")" -ge "4"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.partner_support_waivers WHERE partner_id='$ZERO_ID' AND classification='SPONSORED';")" = "1"
echo ok

echo 'Central-6 Licensing/Finance/Onboarding runtime acceptance passed'
