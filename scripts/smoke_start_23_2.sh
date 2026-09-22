#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start232-owner.txt"
BODY="$TMP_ROOT/himate-start232-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
GROUP_KEY="ci_232_$STAMP"
MODULE_KEY="ci.start232_$STAMP"
TARGET_KEY="ci.start232_target_$STAMP"
PARTNER_EMAIL="start232-$STAMP@example.com"
PARTNER_DOMAIN="start232-$STAMP.example.com"

TODAY="$(python3 - <<'PY'
from datetime import datetime,timezone
print(datetime.now(timezone.utc).date().isoformat())
PY
)"
NEXT_MONTH="$(python3 - "$TODAY" <<'PY'
from datetime import date
import sys
d=date.fromisoformat(sys.argv[1])
print(date(d.year+1,1,1).isoformat() if d.month==12 else date(d.year,d.month+1,1).isoformat())
PY
)"

printf 'START-23.2 owner login... '
payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'create partner and module registry records... '
partner_payload="$(python3 - "$PARTNER_EMAIL" "$PARTNER_DOMAIN" <<'PY'
import json,sys
print(json.dumps({
  "display_name":"START 23.2 Commercial Partner",
  "legal_name":"START 23.2 Commercial Partner LLC",
  "brand_name":"START 23.2",
  "contact_name":"Commercial Owner",
  "contact_email":sys.argv[1],
  "country":"US",
  "primary_domain":sys.argv[2]
}))
PY
)"
partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
test -n "$partner_id"

curl -fsS -b "$COOKIE" -H 'Content-Type: application/json'   -d "$(python3 - "$GROUP_KEY" <<'PY'
import json,sys
print(json.dumps({"group_key":sys.argv[1],"label":"START 23.2 Commercial","sort_order":232}))
PY
)" "$BASE_URL/api/v1/module-groups" >/dev/null

for spec in "$MODULE_KEY|Commercial Matrix Module|40|300" "$TARGET_KEY|Commercial Target Module|10|20"; do
  key="$(printf '%s' "$spec" | cut -d'|' -f1)"
  label="$(printf '%s' "$spec" | cut -d'|' -f2)"
  price="$(printf '%s' "$spec" | cut -d'|' -f3)"
  fee="$(printf '%s' "$spec" | cut -d'|' -f4)"
  body="$(python3 - "$key" "$label" "$GROUP_KEY" "$price" "$fee" <<'PY'
import json,sys
print(json.dumps({
 "key":sys.argv[1],"label":sys.argv[2],"group_key":sys.argv[3],
 "description":"START-23.2 commercial control-plane acceptance",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0",
 "default_monthly_price":float(sys.argv[4]),"default_activation_fee":float(sys.argv[5]),
 "availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY","module_type":"FEATURE","owner_team":"Platform",
 "manifest":{"schema_version":1}
}))
PY
)"
  curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$body" "$BASE_URL/api/v1/modules" >/dev/null
done

curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"default_monthly_price":45,"default_activation_fee":350,"latest_version":"1.0.1"}'   "$BASE_URL/api/v1/modules/$MODULE_KEY" >/dev/null

modules="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/modules")"
printf '%s' "$modules" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["default_monthly_price"]==45,m; assert m["default_activation_fee"]==350,m; assert m["latest_version"]=="1.0.1",m' "$MODULE_KEY"
echo ok

printf 'module relationship create/read/delete works through the canonical Modules control plane... '
relation_body="$(python3 - "$TARGET_KEY" <<'PY'
import json,sys
print(json.dumps({"target_module_key":sys.argv[1],"relation_type":"REQUIRES","note":"START-23.2 dependency canary"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$relation_body" "$BASE_URL/api/v1/modules/$MODULE_KEY/relationships" >/dev/null
relations="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/modules/$MODULE_KEY/relationships")"
printf '%s' "$relations" | python3 -c 'import json,sys; d=json.load(sys.stdin); target=sys.argv[1]; assert any(x["target_module_key"]==target and x["relation_type"]=="REQUIRES" for x in d["items"]),d' "$TARGET_KEY"
curl -fsS -b "$COOKIE" -X DELETE "$BASE_URL/api/v1/modules/$MODULE_KEY/relationships/$TARGET_KEY?type=REQUIRES" >/dev/null
echo ok

printf 'partner commercial terms and agreement persist... '
terms="$(python3 - "$TODAY" <<'PY'
import json,sys
print(json.dumps({
 "currency":"USD","activation_fee":13000,"activation_fee_waived":False,"activation_fee_reason":"",
 "base_monthly_fee":100,"annual_increase_percent":0,
 "price_effective_from":sys.argv[1],"service_anchor_date":sys.argv[1],
 "reason":"START-23.2 commercial terms"
}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$terms" "$BASE_URL/api/v1/billing/partners/$partner_id/terms" >/dev/null
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json'   -d '{"status":"AGREED","agreement_reference":"contract://start232/agreed","note":"START-23.2 commercial agreement"}'   "$BASE_URL/api/v1/billing/partners/$partner_id/agreement" >/dev/null
terms_read="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/terms")"
printf '%s' "$terms_read" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["base_monthly_fee"]==100 and d["billing_cycle_model"]=="CALENDAR_MONTH" and d["cycle_days"] is None,d'
agreement="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/agreement")"
printf '%s' "$agreement" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="AGREED" and d["agreement_reference"]=="contract://start232/agreed",d'
echo ok

printf 'partner-specific recurring price and activation fee persist in the commercial matrix... '
assignment="$(python3 - "$TODAY" <<'PY'
import json,sys
print(json.dumps({
 "status":"ACTIVE","visible":True,"included_in_base":False,
 "partner_price":75,"price_effective_at":sys.argv[1]+"T00:00:00Z",
 "partner_activation_fee":250,"activation_fee_effective_at":sys.argv[1]+"T00:00:00Z",
 "reason":"START-23.2 partner-specific commercial override"
}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$assignment" "$BASE_URL/api/v1/partners/$partner_id/modules/$MODULE_KEY" >/dev/null

matrix="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/module-commercial-matrix?partner_ids=$partner_id")"
printf '%s' "$matrix" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; p=sys.argv[2]; m=next(x for x in d["items"] if x["partner_id"]==p and x["key"]==key); assert m["status"]=="ACTIVE",m; assert m["visible"] is True,m; assert m["included_in_base"] is False,m; assert m["default_monthly_price"]==45,m; assert m["partner_price"]==75,m; assert m["default_activation_fee"]==350,m; assert m["partner_activation_fee"]==250,m; assert m["price_source"]=="PARTNER_HISTORY",m; assert m["activation_fee_source"]=="PARTNER_HISTORY",m' "$MODULE_KEY" "$partner_id"

history="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules/$MODULE_KEY/commercial-history")"
printf '%s' "$history" | python3 -c 'import json,sys; d=json.load(sys.stdin); fields={x["field"] for x in d["items"]}; required={"status","visible","partner_price","partner_activation_fee"}; assert required <= fields,(required-fields,d)'
echo ok

printf 'billing creates immutable current calendar-month subscription and exact next-month quote... '
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/summary" >/dev/null
subs="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/subscription-matrix?partner_ids=$partner_id")"
printf '%s' "$subs" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; s=next(x for x in d["items"] if x["module_key"]==key); assert s["price"]==75,s; assert s["current_period_included_in_base"] is False,s; assert s["next_period_price"]==75,s; assert s["next_period_included_in_base"] is False,s; assert s["next_billing_date"]==s["period_end_exclusive"],s' "$MODULE_KEY"
echo ok

printf 'future commercial changes remain scheduled while current calendar month stays immutable... '
future="$(python3 - "$NEXT_MONTH" <<'PY'
import json,sys
print(json.dumps({
 "partner_price":90,"price_effective_at":sys.argv[1]+"T00:00:00Z",
 "partner_activation_fee":500,"activation_fee_effective_at":sys.argv[1]+"T00:00:00Z",
 "reason":"START-23.2 next-period commercial schedule"
}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$future" "$BASE_URL/api/v1/partners/$partner_id/modules/$MODULE_KEY" >/dev/null

matrix_next="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/module-commercial-matrix?partner_ids=$partner_id")"
printf '%s' "$matrix_next" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["partner_price"]==75,m; assert m["partner_activation_fee"]==250,m; assert m["next_partner_price"]==90,m; assert m["next_partner_activation_fee"]==500,m' "$MODULE_KEY"

subs_next="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/subscription-matrix?partner_ids=$partner_id")"
printf '%s' "$subs_next" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; s=next(x for x in d["items"] if x["module_key"]==key); assert s["price"]==75,s; assert s["next_period_price"]==90,s' "$MODULE_KEY"
echo ok

printf 'commercial mutation is visible in central audit... '
sleep 1
audit="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/audit/events?resource=catalog&limit=100")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert any(x.get("resource")=="catalog" and x.get("method")=="PATCH" and x.get("outcome")=="SUCCESS" for x in d.get("items",[])),d'
echo ok

echo "HIMATE START-23.2 Partner × Module Commercial Control Plane smoke passed"
