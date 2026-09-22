#!/usr/bin/env sh
set -eu

. scripts/payment_test_helpers.sh

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start239-owner.txt"
REPORT_COOKIE="$TMP_ROOT/himate-start239-reporting.txt"
BODY="$TMP_ROOT/himate-start239-body.json"
rm -f "$OWNER_COOKIE" "$REPORT_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$REPORT_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
TODAY="$(date -u +%Y-%m-%d)"
YEAR="$(date -u +%Y)"
MONTH="$(date -u +%-m 2>/dev/null || date -u +%m | sed 's/^0//')"
METRIC_KEY="klavierhaus.events.attendance.attendee_count"
REPORT_EMAIL="start239-reporting-$STAMP@example.com"
REPORT_PASSWORD="Start239!Reporting$STAMP"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-23.9 owner login and baseline dashboard... '
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null
before="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
printf '%s' "$before" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "billing" in d and "impact" in d and "activity" in d; assert len(d["impact"]["trend"])==12'
before_revenue="$(printf '%s' "$before" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(next((float(x["revenue_ytd"]) for x in d["billing"]["items"] if x["currency"]=="USD"),0))')"
before_people="$(printf '%s' "$before" | python3 -c 'import json,sys; print(float(json.load(sys.stdin)["impact"]["people_reached_ytd"]))')"
before_month="$(printf '%s' "$before" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=int(sys.argv[1]); print(float(next(x["value"] for x in d["impact"]["trend"] if int(x["month"])==m)))' "$MONTH")"
echo ok

printf 'create searchable partner and provider-settled activation revenue... '
partner_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({
 "display_name":"START239 Search "+s,
 "legal_name":"START239 Search LLC "+s,
 "brand_name":"START239 "+s,
 "contact_name":"Dashboard Owner",
 "contact_email":"start239-"+s+"@example.com",
 "country":"US"
}))
PY
)"
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
terms="$(python3 - "$TODAY" <<'PY'
import json,sys
day=sys.argv[1]
print(json.dumps({
 "currency":"USD","activation_fee":13000,"activation_fee_waived":False,"activation_fee_reason":"",
 "base_monthly_fee":0,"annual_increase_percent":0,
 "price_effective_from":day,"service_anchor_date":day,
 "reason":"START-23.9 dashboard revenue acceptance"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$terms" "$BASE_URL/api/v1/billing/partners/$partner_id/terms" >/dev/null
provider_pay_activation "$BASE_URL" "$OWNER_COOKIE" "$partner_id" "start239_$STAMP"
after_revenue_json="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
printf '%s' "$after_revenue_json" | python3 -c 'import json,sys; d=json.load(sys.stdin); before=float(sys.argv[1]); x=next(i for i in d["billing"]["items"] if i["currency"]=="USD"); assert float(x["revenue_ytd"]) >= before+12999.99,(before,x)' "$before_revenue"
echo ok

printf 'ensure authoritative People Reached metric and mutate current-month Impact... '
definitions="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/impact/definitions")"
if ! printf '%s' "$definitions" | python3 -c 'import json,sys; key=sys.argv[1]; raise SystemExit(0 if any(x["metric_key"]==key for x in json.load(sys.stdin)["items"]) else 1)' "$METRIC_KEY"; then
  metric_payload="$(python3 - "$METRIC_KEY" <<'PY'
import json,sys
print(json.dumps({
 "metric_key":sys.argv[1],
 "label_en":"People Reached","label_hu":"Elért emberek",
 "description_en":"Authoritative digital attendance count",
 "description_hu":"Hiteles digitális látogatottsági darabszám",
 "unit":"count","aggregation":"SUM","scope":"PARTNER"
}))
PY
)"
  curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$metric_payload" "$BASE_URL/api/v1/impact/definitions" >/dev/null
fi
for value in 17 23; do
  impact_payload="$(python3 - "$partner_id" "$METRIC_KEY" "$TODAY" "$value" "$STAMP" <<'PY'
import json,sys
partner,key,day,value,stamp=sys.argv[1:]
print(json.dumps({
 "partner_id":partner,"metric_key":key,"period_start":day,"period_end":day,
 "numeric_value":float(value),"provenance":"MANUAL",
 "source_ref":"start-23.9-dashboard-"+stamp+"-"+value
}))
PY
)"
  curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$impact_payload" "$BASE_URL/api/v1/impact/values" >/dev/null
done
after_impact="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
printf '%s' "$after_impact" | python3 -c 'import json,sys; d=json.load(sys.stdin); before=float(sys.argv[1]); before_m=float(sys.argv[2]); month=int(sys.argv[3]); assert float(d["impact"]["people_reached_ytd"]) >= before+40; x=next(i for i in d["impact"]["trend"] if int(i["month"])==month); assert float(x["value"]) >= before_m+40; assert d["impact"]["metric_key"]=="klavierhaus.events.attendance.attendee_count"' "$before_people" "$before_month" "$MONTH"
echo ok

printf 'create searchable module, CMS page, contact lead and restricted administrator... '
group_key="ci_239_$STAMP"
module_key="ci.start239_$STAMP"
group_payload="$(python3 - "$group_key" "$STAMP" <<'PY'
import json,sys
print(json.dumps({"group_key":sys.argv[1],"label_en":"START239 Group "+sys.argv[2],"label_hu":"START239 Csoport "+sys.argv[2],"sort_order":239}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$group_payload" "$BASE_URL/api/v1/module-groups" >/dev/null
module_payload="$(python3 - "$module_key" "$group_key" "$STAMP" <<'PY'
import json,sys
key,group,s=sys.argv[1:]
print(json.dumps({
 "key":key,"group_key":group,"label_en":"START239 Search "+s,"label_hu":"START239 Keresés "+s,
 "description_en":"Dashboard global search acceptance","description_hu":"Dashboard globális keresési elfogadás",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0",
 "default_monthly_price":0,"default_activation_fee":0,
 "availability":"ACTIVE","module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$module_payload" "$BASE_URL/api/v1/modules" >/dev/null

page_key="ci_239_$STAMP"
slug="ci-239-$STAMP"
cms_payload="$(python3 - "$page_key" "$slug" "$STAMP" <<'PY'
import json,sys
key,slug,s=sys.argv[1:]
print(json.dumps({
 "page_key":key,"name":"START239 Search "+s,"locale":"en_US",
 "version":{
  "slug":slug,
  "seo":{"title":"START239 Search "+s,"meta_description":"START 23.9 global search acceptance page.","keywords":["start239"],"canonical":"https://www.himate.com/"+slug,"og_title":"START239 Search","og_description":"Search acceptance","og_image_asset_id":"","noindex":True},
  "sections":[{"id":"search-proof","component_type":"TEXT","heading":"START239 "+s,"body":"Global search proof","media_asset_id":"","cta_label":"","cta_url":"","visible":True,"sort_order":10,"settings":{}}]
 }
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$cms_payload" "$BASE_URL/api/v1/cms/pages" >/dev/null

contact_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({"name":"START239 Search "+s,"organization":"START239 Search "+s,"email":"contact-"+s+"@example.com","message":"START239 global search acceptance message "+s}))
PY
)"
curl -fsS -H 'Content-Type: application/json' -d "$contact_payload" "$BASE_URL/api/v1/public/contact" >/dev/null

admin_payload="$(python3 - "$STAMP" "$REPORT_EMAIL" "$REPORT_PASSWORD" <<'PY'
import json,sys
s,email,password=sys.argv[1:]
print(json.dumps({"name":"START239 Search "+s,"email":email,"password":password,"roles":["reporting_admin"]}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$admin_payload" "$BASE_URL/api/v1/admin/users" >/dev/null
echo ok

printf 'owner global search returns all permitted authoritative domains... '
owner_search="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/search?q=$STAMP&limit=5")"
printf '%s' "$owner_search" | python3 -c 'import json,sys; d=json.load(sys.stdin); resources={x["resource"] for x in d["items"]}; required={"partners","catalog","cms","contact","administration"}; assert required<=resources,(required-resources,d); assert d["permission_scoped"] is True'
echo ok

printf 'restricted Reporting search cannot leak forbidden domains... '
report_login="$(python3 - "$REPORT_EMAIL" "$REPORT_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$REPORT_COOKIE" -H 'Content-Type: application/json' -d "$report_login" "$BASE_URL/api/v1/auth/login" >/dev/null
restricted="$(curl -fsS -b "$REPORT_COOKIE" "$BASE_URL/api/v1/search?q=$STAMP&limit=5")"
printf '%s' "$restricted" | python3 -c 'import json,sys; d=json.load(sys.stdin); resources={x["resource"] for x in d["items"]}; assert "partners" in resources,d; forbidden={"catalog","cms","contact","administration","audit"}; assert not (resources & forbidden),(resources & forbidden,d)'
test "$(status "$REPORT_COOKIE" GET "/api/v1/search?q=x")" = "400"
grep -q 'Search query must contain at least 2 characters' "$BODY"
echo ok

printf 'Recent Activity is real, permission-filtered audit data... '
sleep 1
activity="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
printf '%s' "$activity" | python3 -c 'import json,sys; d=json.load(sys.stdin); a=d["activity"]; assert a["source"]=="IDENTITY_APPEND_ONLY_AUDIT"; assert a["count"]>0; assert any(x["resource"] in {"partners","catalog","cms","administration","impact","billing"} for x in a["items"]),a'
echo ok

printf 'historical Dashboard/search placeholders are absent... '
if grep -q 'Billing analytics upcoming\|Impact data in START-13\|Global search will be activated in a later functional cycle\.\|final vals=<double>\[\.12,.26,.20,.37,.49,.39,.53,.48,.61,.70,.68,.84\]' frontend/lib/main.dart; then
  echo "START-23.9 placeholder remains in Flutter source" >&2
  exit 1
fi
echo ok

echo "HIMATE START-23.9 Dashboard, Analytics & Global Search smoke passed"
