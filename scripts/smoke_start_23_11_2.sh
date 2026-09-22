#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start23112-owner.txt"
rm -f "$COOKIE"
trap 'rm -f "$COOKIE"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
MODULE_KEY="ci.start23112_$STAMP"

read MONTH_START MID_MONTH NEXT_MONTH <<EOF
$(python3 - <<'PY'
from datetime import date
today=date.today()
start=today.replace(day=1)
mid=start.replace(day=min(15, (date(start.year + (start.month==12), 1 if start.month==12 else start.month+1, 1)-start).days))
if start.month==12: nxt=date(start.year+1,1,1)
else: nxt=date(start.year,start.month+1,1)
print(start.isoformat(), mid.isoformat(), nxt.isoformat())
PY
)
EOF

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'create START-23.11.2 partner and published module... '
partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.11.2 Calendar Partner","legal_name":"START 23.11.2 Calendar Partner LLC","brand_name":"23112","contact_name":"Billing Owner","contact_email":"start23112@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
module_payload="$(python3 - "$MODULE_KEY" <<'PY'
import json,sys
print(json.dumps({
 "key":sys.argv[1],"group_key":"technical","label_en":"START 23.11.2 Calendar Module","label_hu":"START 23.11.2 Naptári Modul",
 "description_en":"Calendar-month no-proration acceptance","description_hu":"Naptári havi teljes díjas elfogadás",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":999,"default_activation_fee":0,
 "availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY","module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$module_payload" "$BASE_URL/api/v1/modules" >/dev/null
echo ok

printf 'configure individual calendar-month commercial terms... '
terms_payload="$(python3 - "$MID_MONTH" <<'PY'
import json,sys
print(json.dumps({"currency":"USD","activation_fee":0,"activation_fee_waived":True,"activation_fee_reason":"CI",
 "base_monthly_fee":850,"minimum_monthly_commitment":1500,"quote_reference":"Q-23112-CALENDAR",
 "annual_increase_percent":0,"price_effective_from":sys.argv[1],"service_anchor_date":sys.argv[1],"reason":"START-23.11.2 calendar month"}))
PY
)"
terms="$(curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$terms_payload" "$BASE_URL/api/v1/billing/partners/$partner_id/terms")"
printf '%s' "$terms" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["billing_cycle_model"]=="CALENDAR_MONTH",d; assert d["invoice_day"]==1,d; assert d["proration"]=="NONE",d; assert d["minimum_monthly_commitment"]==1500,d'
echo ok

printf 'activate module with full monthly partner price and force mid-month activation evidence... '
commercial_payload='{"status":"ACTIVE","visible":true,"included_in_base":false,"contract_currency":"USD","quote_reference":"Q-23112-CALENDAR","partner_price":275,"partner_activation_fee":0,"reason":"START-23.11.2 full month"}'
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$commercial_payload" "$BASE_URL/api/v1/partners/$partner_id/modules/$MODULE_KEY" >/dev/null
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE catalog.partner_modules SET activated_at='$MID_MONTH'::date WHERE partner_id='$partner_id' AND module_key='$MODULE_KEY';" >/dev/null
summary="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/summary")"
printf '%s' "$summary" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["billing_cycle_model"]=="CALENDAR_MONTH",d; assert d["proration"]=="NONE",d; assert d["current_period_start"]==sys.argv[1],d; assert d["current_period_end_exclusive"]==sys.argv[2],d; assert d["extra_module_fee"]==275,d; assert d["current_total"]==1500,d' "$MONTH_START" "$NEXT_MONTH"
subs="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions")"
printf '%s' "$subs" | python3 -c 'import json,sys; d=json.load(sys.stdin); key,start,end=sys.argv[1:]; s=next(x for x in d["items"] if x["module_key"]==key); assert s["billing_model"]=="CALENDAR_MONTH",s; assert s["proration"]=="NONE",s; assert s["period_start"]==start,s; assert s["period_end_exclusive"]==end,s; assert s["price"]==275,s' "$MODULE_KEY" "$MONTH_START" "$NEXT_MONTH"
item_amount="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT amount FROM billing.invoice_items WHERE partner_id='$partner_id' AND module_key='$MODULE_KEY' AND item_type='MODULE' AND billing_model='CALENDAR_MONTH' AND period_start='$MONTH_START'::date;")"
test "$item_amount" = "275.00"
echo ok

printf 'schedule cancellation at the calendar-month boundary... '
cancelled="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"cancel_at_period_end":true,"reason":"START-23.11.2 month boundary"}' "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions/$MODULE_KEY")"
printf '%s' "$cancelled" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle_state"]=="CANCEL_PENDING",d; assert d["cancellation_effective_at"]==sys.argv[1],d' "$NEXT_MONTH"
echo ok

printf 'day-1 cycle invoices the completed calendar month and enforces the minimum commitment... '
docker compose exec -T billing /app/service --run-invoice-cycle "$NEXT_MONTH"
invoices="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/invoices")"
invoice_id="$(printf '%s' "$invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); start,end=sys.argv[1:]; x=next(i for i in d["items"] if str(i["service_period_start"])[:10]==start and str(i["service_period_end_exclusive"])[:10]==end and i["billing_model"]=="CALENDAR_MONTH"); assert x["base_fee"]==850,x; assert x["module_fee"]==275,x; assert x["minimum_commitment_adjustment"]==375,x; assert x["total"]==1500,x; types={i["item_type"] for i in x["items"]}; assert {"BASE_SERVICE","MODULE","MINIMUM_COMMITMENT"} <= types,(types,x); print(x["id"])' "$MONTH_START" "$NEXT_MONTH")"
test -n "$invoice_id"
echo ok

printf 'calendar invoice rerun is idempotent and ledger commercial values are immutable... '
docker compose exec -T billing /app/service --run-invoice-cycle "$NEXT_MONTH"
count="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.invoices WHERE partner_id='$partner_id' AND billing_model='CALENDAR_MONTH' AND service_period_start='$MONTH_START'::date AND service_period_end='$NEXT_MONTH'::date;")"
test "$count" = "1"
if docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE billing.invoice_items SET amount=999 WHERE invoice_id='$invoice_id' AND item_type='MODULE';" >/dev/null 2>&1; then
  echo 'calendar-month invoice item mutation unexpectedly succeeded' >&2
  exit 1
fi
state="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/subscriptions")"
printf '%s' "$state" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; s=next(x for x in d["items"] if x["module_key"]==key); assert s["lifecycle_state"]=="INACTIVE",s' "$MODULE_KEY"
echo ok

echo 'HIMATE START-23.11.2 calendar-month billing smoke passed'
