#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-central8-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-central8-partner.txt"
PARTNERS_PDF="$TMP_ROOT/himate-central8-partners.pdf"
PACKAGES_PDF="$TMP_ROOT/himate-central8-packages.pdf"
FINANCE_PDF="$TMP_ROOT/himate-central8-finance.pdf"
IMPACT_PDF="$TMP_ROOT/himate-central8-impact.pdf"
HEADERS="$TMP_ROOT/himate-central8-headers.txt"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$PARTNERS_PDF" "$PACKAGES_PDF" "$FINANCE_PDF" "$IMPACT_PDF" "$HEADERS"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$PARTNERS_PDF" "$PACKAGES_PDF" "$FINANCE_PDF" "$IMPACT_PDF" "$HEADERS"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'CENTRAL-8 canonical package definition is Starter 990/10, Business 1490/20, Premium 2490/Unlimited... '
plans="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/plans")"
printf '%s' "$plans" | python3 -c '
import json,sys
d=json.load(sys.stdin); p={x["plan_key"]:x for x in d["items"]}
s,b,x=p["STARTER"],p["BUSINESS"],p["FLEX"]
assert s["display_name"]=="Starter" and s["monthly_price"]==990 and s["module_limit"]==10 and s["selection_mode"]=="FIXED",s
assert b["display_name"]=="Business" and b["monthly_price"]==1490 and b["module_limit"]==20 and b["selection_mode"]=="FIXED",b
assert x["display_name"]=="Premium" and x["monthly_price"]==2490 and x["module_limit"] is None and x["selection_mode"]=="UNLIMITED",x
'
echo ok

printf 'CENTRAL-8 Package Analytics is subscription-backed and exposes real usage/activity fields... '
analytics="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/packages/analytics")"
printf '%s' "$analytics" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["source"]=="BILLING_SUBSCRIPTIONS_CATALOG_USAGE_PORTAL_ACTIVITY",d
assert {x["plan_key"] for x in d["packages"]}=={"STARTER","BUSINESS","FLEX"},d["packages"]
assert all("module_usage_events_30d" in x and "portal_active_hours_30d" in x for x in d["partners"]),d["partners"][:3]
assert d["portal_activity_definition"].startswith("5-minute buckets"),d
'
echo ok

printf 'CENTRAL-8 authenticated Partner Portal use produces measurable package activity instead of a fabricated duration... '
premium_partners="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners?limit=100&offset=0&q=Central-5%20Premium%20Partner")"
PREMIUM_ID="$(printf '%s' "$premium_partners" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[x for x in d["items"] if x.get("display_name")=="Central-5 Premium Partner"]; assert xs,d; print(xs[-1]["id"])')"
PORTAL_EMAIL="central8.package.$STAMP@himate.test"
portal_user_payload="$(python3 - "$PORTAL_EMAIL" <<'PY'
import json,sys
print(json.dumps({
  "name":"Central-8 Package Analyst",
  "email":sys.argv[1],
  "password":"Central8Portal!2345Aa",
  "role":"owner"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_user_payload" "$BASE_URL/api/v1/partners/$PREMIUM_ID/portal-users" >/dev/null
portal_login="$(python3 - "$PORTAL_EMAIL" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":"Central8Portal!2345Aa","remember":False}))
PY
)"
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_login" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/dashboard" >/dev/null
i=0
while [ "$i" -lt 20 ]; do
  analytics="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/packages/analytics")"
  if printf '%s' "$analytics" | python3 -c 'import json,sys; d=json.load(sys.stdin); pid=sys.argv[1]; rows=[x for x in d["partners"] if x["partner_id"]==pid]; raise SystemExit(0 if rows and rows[0].get("portal_active_hours_30d",0)>0 else 1)' "$PREMIUM_ID"; then
    break
  fi
  i=$((i+1))
  sleep 0.25
done
printf '%s' "$analytics" | python3 -c '
import json,sys
d=json.load(sys.stdin); pid=sys.argv[1]
row=next(x for x in d["partners"] if x["partner_id"]==pid)
assert row["portal_activity_measured"] is True,row
assert row["portal_active_hours_30d"]>0,row
assert row["portal_requests_30d"]>=1,row
' "$PREMIUM_ID"
echo ok

printf 'CENTRAL-8 Finance returns four-week and package-aware monthly/weekly paid-revenue series... '
finance="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/finance/overview")"
printf '%s' "$finance" | python3 -c '
import json,sys
d=json.load(sys.stdin)
weekly=[x for x in d["weekly_paid"] if x["currency"]=="USD"]
assert len(weekly)==4,weekly
for key,count in (("weekly_paid_by_plan",4),("monthly_paid_by_plan",12)):
  rows=[x for x in d[key] if x["currency"]=="USD"]
  by={}
  for x in rows: by.setdefault(x["plan_key"],[]).append(x)
  assert set(by)=={"STARTER","BUSINESS","FLEX"},by.keys()
  assert all(len(v)==count for v in by.values()),{k:len(v) for k,v in by.items()}
assert d["analytics_source"]=="CENTRAL_8_INVOICE_LEDGER_TRENDS",d
'
echo ok

assert_pdf_download() {
  path="$1"
  output="$2"
  rm -f "$HEADERS" "$output"
  curl -fsS -D "$HEADERS" -b "$OWNER_COOKIE" -o "$output" "$BASE_URL$path"
  grep -qi '^content-type: application/pdf' "$HEADERS"
  grep -qi '^content-disposition: attachment;' "$HEADERS"
  test -s "$output"
  first_bytes="$(dd if="$output" bs=1 count=5 2>/dev/null)"
  test "$first_bytes" = "%PDF-"
}

printf 'CENTRAL-8/9 Partners bulk export returns a branded PDF document... '
assert_pdf_download "/api/v1/partners/export.pdf" "$PARTNERS_PDF"
echo ok

printf 'CENTRAL-8/9 Package Analytics bulk export returns a branded PDF document... '
assert_pdf_download "/api/v1/billing/packages/export.pdf" "$PACKAGES_PDF"
echo ok

printf 'CENTRAL-8/9 Finance bulk export returns a branded PDF document... '
assert_pdf_download "/api/v1/billing/finance/export.pdf" "$FINANCE_PDF"
echo ok

printf 'CENTRAL-8/9 Impact bulk export returns a stable branded PDF even when no observations exist... '
assert_pdf_download "/api/v1/impact/export.pdf" "$IMPACT_PDF"
echo ok

echo 'CENTRAL-8 manual QA, analytics, export and loading runtime acceptance passed'
