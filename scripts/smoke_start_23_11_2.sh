#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start23112-owner.txt"
BODY="$TMP_ROOT/himate-start23112-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
PREFIX="ci.plan.$STAMP"

read TODAY NEXT_MONTH MONTH_AFTER_NEXT DUNNING_DAY3 DUNNING_DAY6 RECOVERY_DAY SECOND_DAY6 SECOND_PURGE_DAY <<EOF
$(python3 - <<'PY'
from datetime import date,timedelta
d=date.today()
n=date(d.year+1,1,1) if d.month==12 else date(d.year,d.month+1,1)
n2=date(n.year+1,1,1) if n.month==12 else date(n.year,n.month+1,1)
print(
 d.isoformat(),n.isoformat(),n2.isoformat(),
 (n+timedelta(days=2)).isoformat(),
 (n+timedelta(days=5)).isoformat(),
 (n+timedelta(days=9)).isoformat(),
 (n2+timedelta(days=5)).isoformat(),
 (n2+timedelta(days=35)).isoformat(),
)
PY
)
EOF

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'create 21 published READY plan modules for 10/20/Unlimited acceptance... '
i=1
while [ "$i" -le 21 ]; do
  key="$PREFIX.$i"
  payload="$(python3 - "$key" "$i" <<'PY'
import json,sys
key=sys.argv[1];i=sys.argv[2]
print(json.dumps({"key":key,"group_key":"client_operations","label_en":"Plan Module "+i,"label_hu":"Plan Modul "+i,
 "description_en":"START-23.11.2 plan acceptance","description_hu":"START-23.11.2 csomag elfogadas",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":0,"default_activation_fee":0,
 "availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY","module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}}))
PY
)"
  curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/modules" >/dev/null
  i=$((i+1))
done
echo ok

STARTER_KEYS="$(python3 - "$PREFIX" <<'PY'
import json,sys;p=sys.argv[1];print(json.dumps([f'{p}.{i}' for i in range(1,21)]))
PY
)"
BUSINESS_KEYS="$(python3 - "$PREFIX" <<'PY'
import json,sys;p=sys.argv[1];print(json.dumps([f'{p}.{i}' for i in range(1,11)]))
PY
)"
printf 'configure fixed Starter and Business packages... '
starter_payload="$(python3 - "$STARTER_KEYS" <<'PY'
import json,sys;print(json.dumps({'fixed_module_keys':json.loads(sys.argv[1])}))
PY
)"
business_payload="$(python3 - "$BUSINESS_KEYS" <<'PY'
import json,sys;print(json.dumps({'fixed_module_keys':json.loads(sys.argv[1])}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$starter_payload" "$BASE_URL/api/v1/billing/plans/STARTER" >/dev/null
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$business_payload" "$BASE_URL/api/v1/billing/plans/BUSINESS" >/dev/null
plans="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/plans")"
printf '%s' "$plans" | python3 -c 'import json,sys; d=json.load(sys.stdin); p={x["plan_key"]:x for x in d["items"]}; s=p["STARTER"]; b=p["BUSINESS"]; premium=p["FLEX"]; assert (s["monthly_price"],s["annual_list_price"],s["annual_price"],s["module_limit"],s["ready"])==(990,11880,11880,10,True),s; assert (b["monthly_price"],b["annual_list_price"],b["annual_price"],b["annual_savings"],b["module_limit"],b["ready"])==(1490,17880,16390,1490,20,True),b; assert premium["display_name"]=="Premium",premium; assert premium["monthly_price"]==2490 and premium["annual_list_price"]==29880 and premium["annual_price"]==22410 and premium["annual_savings"]==7470,premium; assert premium["module_limit"] is None and premium["selection_mode"]=="UNLIMITED" and premium["unlimited_modules"] is True,premium'
echo ok

printf 'create monthly-plan partner and prove activation-license gate... '
partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.11.2 Monthly Partner","legal_name":"START 23.11.2 Monthly Partner LLC","brand_name":"Plan Monthly","contact_name":"Plan Owner","contact_email":"plan-monthly@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
blocked_code="$(status "$COOKIE" PATCH "/api/v1/billing/partners/$partner_id/plan" -H 'Content-Type: application/json' -d '{"plan_key":"STARTER","billing_frequency":"MONTHLY"}')"
test "$blocked_code" = "409"
grep -q 'ACTIVATION_LICENSE_REQUIRED' "$BODY"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":0,"waived":true,"waiver_reason":"START-23.11.2 CI activation gate"}' "$BASE_URL/api/v1/billing/partners/$partner_id/license" >/dev/null
autopay_blocked="$(status "$COOKIE" PATCH "/api/v1/billing/partners/$partner_id/plan" -H 'Content-Type: application/json' -d '{"plan_key":"STARTER","billing_frequency":"MONTHLY"}')"
test "$autopay_blocked" = "409"
grep -q 'AUTOPAY_REQUIRED' "$BODY"
monthly_customer="cus_start23112_monthly_$STAMP"
monthly_method="pm_start23112_monthly_$STAMP"
monthly_profile="$(python3 - "$monthly_customer" "$monthly_method" <<'PY'
import json,sys
print(json.dumps({"provider_customer_id":sys.argv[1],"payment_method_id":sys.argv[2],"autopay_enabled":True},separators=(",",":")))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$monthly_profile" "$BASE_URL/api/v1/payments/partners/$partner_id/profile" >/dev/null
starter="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"plan_key":"STARTER","billing_frequency":"MONTHLY","reason":"START-23.11.2 initial Starter"}' "$BASE_URL/api/v1/billing/partners/$partner_id/plan")"
printf '%s' "$starter" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["plan_key"]=="STARTER",d; assert d["billing_frequency"]=="MONTHLY",d; assert d["monthly_price"]==990,d; assert len(d["active_module_keys"])==10,d'
echo ok

printf 'same-frequency upgrades charge full price difference and apply immediately... '
business_code="$(status "$COOKIE" PATCH "/api/v1/billing/partners/$partner_id/plan" -H 'Content-Type: application/json' -d '{"plan_key":"BUSINESS","billing_frequency":"MONTHLY","reason":"START-23.11.2 upgrade"}')"
if [ "$business_code" != "200" ]; then
  echo "Business upgrade returned HTTP $business_code: $(cat "$BODY")" >&2
  docker compose logs --no-color --tail=120 billing catalog >&2 || true
  exit 1
fi
business="$(cat "$BODY")"
printf '%s' "$business" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["plan_key"]=="BUSINESS" and d["change_type"]=="IMMEDIATE_UPGRADE" and d["upgrade_charge"]==500,d; assert len(d["active_module_keys"])==20,d'
premium="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"plan_key":"FLEX","billing_frequency":"MONTHLY","reason":"START-23.11.2 Premium Unlimited upgrade"}' "$BASE_URL/api/v1/billing/partners/$partner_id/plan")"
printf '%s' "$premium" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["plan_key"]=="FLEX" and d["display_name"]=="Premium" and d["change_type"]=="IMMEDIATE_UPGRADE" and d["upgrade_charge"]==1000,d; assert d["selection_mode"]=="UNLIMITED" and d["module_limit"] is None,d'
ELIGIBLE_COUNT="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/modules" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for x in d["items"] if x.get("publication_status")=="PUBLISHED" and x.get("implementation_state")=="READY" and x.get("availability")=="ACTIVE"))')"
active_count="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for x in d["items"] if x["status"]=="ACTIVE"))')"
test "$active_count" = "$ELIGIBLE_COUNT"
echo ok

printf 'Premium to Business downgrade schedules next month with no immediate entitlement loss... '
downgrade="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"plan_key":"BUSINESS","billing_frequency":"MONTHLY","reason":"START-23.11.2 downgrade"}' "$BASE_URL/api/v1/billing/partners/$partner_id/plan")"
printf '%s' "$downgrade" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["plan_key"]=="FLEX" and d["change_type"]=="SCHEDULED_DOWNGRADE",d; s=d["scheduled_change"]; assert s["next_plan_key"]=="BUSINESS" and s["effective_at"]==sys.argv[1],d' "$NEXT_MONTH"
test "$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for x in d["items"] if x["status"]=="ACTIVE"))')" = "$ELIGIBLE_COUNT"
echo ok

printf 'next-month billing applies Business downgrade and creates one PLAN invoice... '
docker compose exec -T billing /app/service --run-invoice-cycle "$NEXT_MONTH"
state="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/plan")"
printf '%s' "$state" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["plan_key"]=="BUSINESS",d; assert d["monthly_price"]==1490,d; assert len(d["active_module_keys"])==20,d'
invoices="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/invoices")"
printf '%s' "$invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[x for x in d["items"] if x.get("billing_model")=="PLAN" and x.get("charge_type")=="PLAN_MONTHLY"]; assert len(xs)==1,xs; x=xs[0]; assert x["plan_key"]=="BUSINESS" and x["net_total"]==1490 and x["tax_amount"]==0 and x["total"]==1490,x; assert [i["item_type"] for i in x["items"]]==["PLAN"],x'
docker compose exec -T billing /app/service --run-invoice-cycle "$NEXT_MONTH"
count="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.invoices WHERE partner_id='$partner_id' AND billing_model='PLAN' AND charge_type='PLAN_MONTHLY' AND service_period_start='$NEXT_MONTH'::date;")"
test "$count" = "1"
echo ok

printf 'recurring payment dunning retries on day 1, 3 and 6 then suspends service... '
monthly_invoice_id="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT id FROM billing.invoices WHERE partner_id='$partner_id' AND billing_model='PLAN' AND charge_type='PLAN_MONTHLY' AND service_period_start='$NEXT_MONTH'::date LIMIT 1;")"
test -n "$monthly_invoice_id"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT collection_attempts FROM billing.invoices WHERE id='$monthly_invoice_id';")" = "1"
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE billing.invoices SET provider_status='FAILED' WHERE id='$monthly_invoice_id';" >/dev/null
docker compose exec -T billing /app/service --run-invoice-cycle "$DUNNING_DAY3"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT collection_attempts FROM billing.invoices WHERE id='$monthly_invoice_id';")" = "2"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT status FROM billing.partner_plan_subscriptions WHERE partner_id='$partner_id';")" = "PAST_DUE"
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE billing.invoices SET provider_status='FAILED' WHERE id='$monthly_invoice_id';" >/dev/null
docker compose exec -T billing /app/service --run-invoice-cycle "$DUNNING_DAY6"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT collection_attempts FROM billing.invoices WHERE id='$monthly_invoice_id';")" = "3"
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE billing.invoices SET provider_status='FAILED' WHERE id='$monthly_invoice_id';" >/dev/null
docker compose exec -T billing /app/service --run-invoice-cycle "$DUNNING_DAY6"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT status FROM billing.partner_plan_subscriptions WHERE partner_id='$partner_id';")" = "SUSPENDED"
test "$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id" | python3 -c 'import json,sys; print(json.load(sys.stdin)["lifecycle"])')" = "SUSPENDED"
test "$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for x in d["items"] if x["status"]=="ACTIVE"))')" = "0"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT purge_due_at FROM billing.invoices WHERE id='$monthly_invoice_id';")" = "$(python3 - "$DUNNING_DAY6" <<'PY'
from datetime import date,timedelta
import sys
print((date.fromisoformat(sys.argv[1])+timedelta(days=30)).isoformat())
PY
)"
echo ok

printf 'paid invoice inside cure window restores plan, lifecycle and entitlements... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE billing.invoices SET status='PAID',provider_status='SUCCEEDED' WHERE id='$monthly_invoice_id';" >/dev/null
docker compose exec -T billing /app/service --run-invoice-cycle "$RECOVERY_DAY"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT status FROM billing.partner_plan_subscriptions WHERE partner_id='$partner_id';")" = "ACTIVE"
test "$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id" | python3 -c 'import json,sys; print(json.load(sys.stdin)["lifecycle"])')" = "PROSPECT"
test "$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for x in d["items"] if x["status"]=="ACTIVE"))')" = "20"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT dunning_state FROM billing.invoices WHERE id='$monthly_invoice_id';")" = "RECOVERED"
echo ok

printf 'expired 30-day cure window archives account and purges operational access while retaining financial ledger... '
docker compose exec -T billing /app/service --run-invoice-cycle "$MONTH_AFTER_NEXT"
second_invoice_id="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT id FROM billing.invoices WHERE partner_id='$partner_id' AND billing_model='PLAN' AND charge_type='PLAN_MONTHLY' AND service_period_start='$MONTH_AFTER_NEXT'::date LIMIT 1;")"
test -n "$second_invoice_id"
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE billing.invoices SET provider_status='FAILED',collection_attempts=3 WHERE id='$second_invoice_id';" >/dev/null
docker compose exec -T billing /app/service --run-invoice-cycle "$SECOND_DAY6"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT status FROM billing.partner_plan_subscriptions WHERE partner_id='$partner_id';")" = "SUSPENDED"
docker compose exec -T billing /app/service --run-invoice-cycle "$SECOND_PURGE_DAY"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT status FROM billing.partner_plan_subscriptions WHERE partner_id='$partner_id';")" = "CANCELLED"
test "$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id" | python3 -c 'import json,sys; print(json.load(sys.stdin)["lifecycle"])')" = "ARCHIVED"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT dunning_state FROM billing.invoices WHERE id='$second_invoice_id';")" = "PURGED"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.invoices WHERE partner_id='$partner_id';")" -ge "1"
echo ok

printf 'create annual Premium partner and verify full list price versus discounted charge... '
annual_partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.11.2 Annual Partner","legal_name":"START 23.11.2 Annual Partner LLC","brand_name":"Plan Annual","contact_name":"Annual Owner","contact_email":"plan-annual@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
annual_id="$(printf '%s' "$annual_partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":0,"waived":true,"waiver_reason":"START-23.11.2 CI annual"}' "$BASE_URL/api/v1/billing/partners/$annual_id/license" >/dev/null
annual_customer="cus_start23112_annual_$STAMP"
annual_method="pm_start23112_annual_$STAMP"
annual_profile="$(python3 - "$annual_customer" "$annual_method" <<'PY'
import json,sys
print(json.dumps({"provider_customer_id":sys.argv[1],"payment_method_id":sys.argv[2],"autopay_enabled":True},separators=(",",":")))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$annual_profile" "$BASE_URL/api/v1/payments/partners/$annual_id/profile" >/dev/null
annual_payload='{"plan_key":"FLEX","billing_frequency":"ANNUAL","reason":"START-23.11.2 annual Premium"}'
annual="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$annual_payload" "$BASE_URL/api/v1/billing/partners/$annual_id/plan")"
printf '%s' "$annual" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["plan_key"]=="FLEX" and d["display_name"]=="Premium" and d["billing_frequency"]=="ANNUAL",d; assert d["annual_list_price"]==29880 and d["annual_price"]==22410 and d["annual_savings"]==7470,d; assert d["selection_mode"]=="UNLIMITED" and d["module_limit"] is None,d'
annual_invoices="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$annual_id/invoices")"
printf '%s' "$annual_invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next(i for i in d["items"] if i.get("charge_type")=="PLAN_ANNUAL_PREPAY"); assert x["list_price"]==29880 and x["discount_amount"]==7470 and x["net_total"]==22410 and x["tax_amount"]==0 and x["total"]==22410,x; assert x["billing_frequency"]=="ANNUAL",x'
echo ok

printf 'Premium rejects finite partner module-set replacement and grows with newly released modules... '
code="$(status "$COOKIE" PUT "/api/v1/billing/partners/$annual_id/plan/modules" -H 'Content-Type: application/json' -d '{"module_keys":[],"reason":"finite override must fail"}')"
test "$code" = "409"
grep -q 'UNLIMITED_PLAN_MANAGED' "$BODY"
future_key="$PREFIX.future"
future_payload="$(python3 - "$future_key" <<'PY'
import json,sys
key=sys.argv[1]
print(json.dumps({"key":key,"group_key":"client_operations","label_en":"Future Premium Module","label_hu":"Jövőbeli Premium Modul",
 "description_en":"Central-5 future module entitlement proof","description_hu":"Central-5 jövőbeli modul jogosultsági bizonyíték",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":0,"default_activation_fee":0,
 "availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY","module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$future_payload" "$BASE_URL/api/v1/modules" >/dev/null
future_state="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$annual_id/modules")"
printf '%s' "$future_state" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["status"]=="ACTIVE" and m["entitlement_state"]=="ACTIVE" and m["plan_key"]=="FLEX",m' "$future_key"
annual_count="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.invoices WHERE partner_id='$annual_id' AND billing_model='PLAN' AND charge_type IN ('PLAN_ANNUAL_PREPAY','PLAN_ANNUAL_RENEWAL');")"
test "$annual_count" = "1"
echo ok

echo 'HIMATE START-23.11.2 subscription-plan recurring billing smoke passed'
