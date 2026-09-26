#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-central8-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-central8-partner.txt"
PARTNERS_CSV="$TMP_ROOT/himate-central8-partners.csv"
PACKAGES_CSV="$TMP_ROOT/himate-central8-packages.csv"
FINANCE_CSV="$TMP_ROOT/himate-central8-finance.csv"
IMPACT_CSV="$TMP_ROOT/himate-central8-impact.csv"
HEADERS="$TMP_ROOT/himate-central8-headers.txt"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$PARTNERS_CSV" "$PACKAGES_CSV" "$FINANCE_CSV" "$IMPACT_CSV" "$HEADERS"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$PARTNERS_CSV" "$PACKAGES_CSV" "$FINANCE_CSV" "$IMPACT_CSV" "$HEADERS"' EXIT

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

assert_csv_download() {
  path="$1"
  output="$2"
  rm -f "$HEADERS" "$output"
  curl -fsS -D "$HEADERS" -b "$OWNER_COOKIE" -o "$output" "$BASE_URL$path"
  grep -qi '^content-type: text/csv' "$HEADERS"
  grep -qi '^content-disposition: attachment;' "$HEADERS"
  test -s "$output"
}

printf 'CENTRAL-8 Partners bulk export returns backend CSV rows... '
assert_csv_download "/api/v1/partners/export.csv" "$PARTNERS_CSV"
python3 - "$PARTNERS_CSV" <<'PY'
import csv,sys
with open(sys.argv[1],newline="",encoding="utf-8") as fh:
    rows=list(csv.DictReader(fh))
assert rows
assert {"partner_id","display_name","lifecycle","system_health"} <= set(rows[0])
assert any(x["partner_id"]=="ptr_000001" for x in rows)
PY
echo ok

printf 'CENTRAL-8 Package Analytics bulk export returns the active package/subscription population... '
assert_csv_download "/api/v1/billing/packages/export.csv" "$PACKAGES_CSV"
python3 - "$PACKAGES_CSV" "$PREMIUM_ID" <<'PY'
import csv,sys
with open(sys.argv[1],newline="",encoding="utf-8") as fh:
    rows=list(csv.DictReader(fh))
assert rows
assert {"partner_id","package","plan_key","module_usage_events_30d","portal_active_hours_30d"} <= set(rows[0])
row=next(x for x in rows if x["partner_id"]==sys.argv[2])
assert row["package"]=="Premium" and row["plan_key"]=="FLEX",row
PY
echo ok

printf 'CENTRAL-8 Finance bulk export returns the invoice-ledger CSV contract... '
assert_csv_download "/api/v1/billing/finance/export.csv" "$FINANCE_CSV"
python3 - "$FINANCE_CSV" <<'PY'
import csv,sys
with open(sys.argv[1],newline="",encoding="utf-8") as fh:
    reader=csv.DictReader(fh)
    assert {"invoice_id","partner_id","workflow_status","net_total","tax_amount","gross_total"} <= set(reader.fieldnames or [])
    rows=list(reader)
assert rows
PY
echo ok

printf 'CENTRAL-8 Impact bulk export has a stable CSV contract even when no observations exist... '
assert_csv_download "/api/v1/impact/export.csv" "$IMPACT_CSV"
python3 - "$IMPACT_CSV" <<'PY'
import csv,sys
with open(sys.argv[1],newline="",encoding="utf-8") as fh:
    reader=csv.DictReader(fh)
    assert {"partner_id","metric_key","period_start","period_end","provenance","recorded_at"} <= set(reader.fieldnames or [])
    list(reader)
PY
echo ok

echo 'CENTRAL-8 manual QA, analytics, export and loading runtime acceptance passed'
