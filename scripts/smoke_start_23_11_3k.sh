#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
EXPECTED_VERSION="${HIMATE_APP_VERSION:-0.8.32-start-23.11.7}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start23113k-owner.txt"
BODY="$TMP_ROOT/himate-start23113k-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
PREFIX="ci.commercial.$STAMP"

read TODAY CHARITY_CYCLE JAN1 DEC1 <<EOF
$(python3 - <<'PY'
from datetime import date,timedelta
today=date.today()
jan1=date(today.year+1,1,1)
print(today.isoformat(),(today+timedelta(days=30)).isoformat(),jan1.isoformat(),date(jan1.year-1,12,1).isoformat())
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

printf 'release is synchronized at START-23.11.3k... '
HEALTH="$(curl -fsS "$BASE_URL/api/v1/health")"
python3 - "$HEALTH" "$EXPECTED_VERSION" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); expected=sys.argv[2]
assert d["status"]=="ok",d
assert d["release_consistent"] is True,d
assert d["version"]==expected,d
PY
echo ok

printf 'create 16 published READY modules for package and Charity acceptance... '
i=1
while [ "$i" -le 16 ]; do
  key="$PREFIX.$i"
  payload="$(python3 - "$key" "$i" <<'PY'
import json,sys
key=sys.argv[1];i=sys.argv[2]
print(json.dumps({
 "key":key,"group_key":"technical","label_en":"Commercial Module "+i,"label_hu":"Kereskedelmi modul "+i,
 "description_en":"START-23.11.3k acceptance","description_hu":"START-23.11.3k elfogadas",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0",
 "default_monthly_price":25,"default_activation_fee":0,
 "availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY",
 "module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}
}))
PY
)"
  curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/modules" >/dev/null
  i=$((i+1))
done
ALL_KEYS="$(python3 - "$PREFIX" <<'PY'
import json,sys;p=sys.argv[1];print(json.dumps([f"{p}.{i}" for i in range(1,17)]))
PY
)"
STARTER_KEYS="$(python3 - "$PREFIX" <<'PY'
import json,sys;p=sys.argv[1];print(json.dumps([f"{p}.{i}" for i in range(1,4)]))
PY
)"
echo ok

printf 'standard package limits are immutable 3 / 10 / 15... '
PLANS="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/plans")"
python3 - "$PLANS" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); p={x["plan_key"]:x for x in d["items"]}
assert p["STARTER"]["module_limit"]==3,p["STARTER"]
assert p["BUSINESS"]["module_limit"]==10,p["BUSINESS"]
assert p["FLEX"]["module_limit"]==15,p["FLEX"]
assert p["STARTER"]["annual_increase_percent"]==5,p["STARTER"]
assert p["BUSINESS"]["annual_increase_percent"]==5,p["BUSINESS"]
assert p["FLEX"]["annual_increase_percent"]==5,p["FLEX"]
PY
limit_code="$(status "$COOKIE" PATCH "/api/v1/billing/plans/STARTER" -H 'Content-Type: application/json' -d '{"module_limit":4,"reason":"must be rejected"}')"
test "$limit_code" = "409"
grep -q 'STANDARD_PACKAGE_LIMIT' "$BODY"
echo ok

printf 'configure the fixed Starter package with exactly three modules... '
starter_modules_payload="$(python3 - "$STARTER_KEYS" <<'PY'
import json,sys;print(json.dumps({"fixed_module_keys":json.loads(sys.argv[1]),"reason":"START-23.11.3k Starter package"}))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$starter_modules_payload" "$BASE_URL/api/v1/billing/plans/STARTER" >/dev/null
echo ok

printf 'legacy commercial terms accept true zero-dollar values... '
comp="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"3k Complimentary Partner","legal_name":"3k Complimentary Partner LLC","contact_email":"3k-comp@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
comp_id="$(printf '%s' "$comp" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
terms_payload="$(python3 - "$TODAY" <<'PY'
import json,sys
print(json.dumps({
 "currency":"USD","activation_fee":0,"base_monthly_fee":0,"minimum_monthly_commitment":0,
 "annual_increase_percent":5,"price_effective_from":sys.argv[1],"service_anchor_date":sys.argv[1],
 "reason":"START-23.11.3k zero-dollar acceptance"
}))
PY
)"
TERMS="$(curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$terms_payload" "$BASE_URL/api/v1/billing/partners/$comp_id/terms")"
python3 - "$TERMS" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["activation_fee"]==0,d
assert d["base_monthly_fee"]==0,d
assert d["minimum_monthly_commitment"]==0,d
assert d["activation_fee_waived"] is True,d
assert d["annual_increase_percent"]==5,d
PY
COMP_MODE="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"billing_mode":"COMPLIMENTARY","reason":"START-23.11.3k complimentary acceptance"}' "$BASE_URL/api/v1/billing/partners/$comp_id/commercial-mode")"
python3 - "$COMP_MODE" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["billing_mode"]=="COMPLIMENTARY",d
assert d["recurring_charge_enabled"] is False,d
assert d["invoice_generation_enabled"] is False,d
PY
echo ok

printf 'Charity cannot be self-enabled before HIMATE approval... '
charity="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"3k Charity Partner","legal_name":"3k Charity Partner Foundation","contact_email":"3k-charity@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
charity_id="$(printf '%s' "$charity" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
charity_terms="$(python3 - "$TODAY" <<'PY'
import json,sys
print(json.dumps({
 "currency":"USD","activation_fee":0,"base_monthly_fee":1500,"minimum_monthly_commitment":0,
 "annual_increase_percent":5,"price_effective_from":sys.argv[1],"service_anchor_date":sys.argv[1],
 "reason":"START-23.11.3k Charity nominal value"
}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$charity_terms" "$BASE_URL/api/v1/billing/partners/$charity_id/terms" >/dev/null
blocked="$(status "$COOKIE" PATCH "/api/v1/billing/partners/$charity_id/commercial-mode" -H 'Content-Type: application/json' -d '{"billing_mode":"CHARITY","reason":"must be blocked before approval"}')"
test "$blocked" = "409"
grep -q 'CHARITY_APPROVAL_REQUIRED' "$BODY"
echo ok

printf 'Charity request becomes pending, then HIMATE approval activates Charity mode... '
REQUESTED="$(curl -fsS -b "$COOKIE" -X POST -H 'Content-Type: application/json' -d '{"reason":"START-23.11.3k eligibility review"}' "$BASE_URL/api/v1/billing/partners/$charity_id/charity/request")"
python3 - "$REQUESTED" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); assert d["charity_status"]=="PENDING",d; assert d["billing_mode"]=="PAID",d
PY
APPROVED="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"charity_status":"APPROVED","reason":"START-23.11.3k HIMATE approval"}' "$BASE_URL/api/v1/billing/partners/$charity_id/commercial-mode")"
python3 - "$APPROVED" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["billing_mode"]=="CHARITY",d
assert d["charity_status"]=="APPROVED",d
assert d["invoice_generation_enabled"] is False,d
assert d["charity_module_selection_enabled"] is True,d
assert d["charity_reviewed_by"],d
PY
echo ok

printf 'approved Charity can select more than Flex maximum with no module-count cap... '
charity_modules_payload="$(python3 - "$ALL_KEYS" <<'PY'
import json,sys
print(json.dumps({"module_keys":json.loads(sys.argv[1]),"reason":"START-23.11.3k unrestricted Charity selection"}))
PY
)"
CHARITY_MODULES="$(curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$charity_modules_payload" "$BASE_URL/api/v1/billing/partners/$charity_id/charity/modules")"
python3 - "$CHARITY_MODULES" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["approved"] is True,d
assert d["count"]==16,d
assert d["module_limit"] is None,d
assert len(d["module_keys"])==16,d
PY
CATALOG="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$charity_id/modules")"
python3 - "$CATALOG" "$PREFIX" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); prefix=sys.argv[2]
xs=[x for x in d["items"] if str(x.get("key","")).startswith(prefix+".")]
assert len(xs)==16,(len(xs),xs)
assert all(x["status"]=="ACTIVE" and x["entitlement_state"]=="ACTIVE" for x in xs),xs
assert all(x.get("entitlement_source")=="CHARITY" for x in xs),xs
PY
echo ok

printf 'Charity 30-day cycle records zero-dollar event and creates no invoice... '
before_count="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.invoices WHERE partner_id='$charity_id';")"
docker compose exec -T billing /app/service --run-invoice-cycle "$CHARITY_CYCLE"
after_count="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.invoices WHERE partner_id='$charity_id';")"
test "$before_count" = "$after_count"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.billing_events WHERE partner_id='$charity_id' AND event_type='ZERO_DOLLAR_BILLING_CYCLE';")" -ge "1"
echo ok

printf 'central Starter price changes to 600 effective today and is auditable... '
price_payload="$(python3 - "$TODAY" <<'PY'
import json,sys
print(json.dumps({
 "monthly_price":600,"annual_list_price":7200,"annual_price":7200,
 "effective_at":sys.argv[1],"reason":"START-23.11.3k mid-year central package increase"
}))
PY
)"
STARTER_PRICE="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$price_payload" "$BASE_URL/api/v1/billing/plans/STARTER")"
python3 - "$STARTER_PRICE" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); assert d["monthly_price"]==600,d; assert d["module_limit"]==3,d; assert d["annual_increase_percent"]==5,d
PY
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.subscription_plan_price_history WHERE plan_key='STARTER' AND monthly_price=600 AND change_type='MANUAL';")" -ge "1"
echo ok

printf 'active paid Starter customer uses the central package price... '
paid="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"3k Paid Starter Partner","legal_name":"3k Paid Starter Partner LLC","contact_email":"3k-paid@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
paid_id="$(printf '%s' "$paid" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":0,"waived":true,"waiver_reason":"START-23.11.3k paid plan activation"}' "$BASE_URL/api/v1/billing/partners/$paid_id/license" >/dev/null
profile="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({"provider_customer_id":"cus_3k_"+s,"payment_method_id":"pm_3k_"+s,"autopay_enabled":True}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$profile" "$BASE_URL/api/v1/payments/partners/$paid_id/profile" >/dev/null
PAID_PLAN="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"plan_key":"STARTER","billing_frequency":"MONTHLY","reason":"START-23.11.3k paid Starter"}' "$BASE_URL/api/v1/billing/partners/$paid_id/plan")"
python3 - "$PAID_PLAN" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); assert d["plan_key"]=="STARTER",d; assert d["monthly_price"]==600,d; assert len(d["active_module_keys"])==3,d
PY
echo ok

printf 'January 1 automatic uplift uses the latest 600 price: 600 x 1.05 = 630... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE billing.partner_plan_subscriptions SET current_period_start='$DEC1'::date,current_period_end='$JAN1'::date,next_billing_at='$JAN1'::date,status='ACTIVE' WHERE partner_id='$paid_id';" >/dev/null
docker compose exec -T billing /app/service --run-invoice-cycle "$JAN1"
annual_price="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT monthly_price FROM billing.subscription_plan_price_history WHERE plan_key='STARTER' AND effective_from='$JAN1'::date AND change_type='ANNUAL_INCREASE' ORDER BY id DESC LIMIT 1;")"
test "$annual_price" = "630.00"
snapshot="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT monthly_price_snapshot FROM billing.partner_plan_subscriptions WHERE partner_id='$paid_id';")"
test "$snapshot" = "630.00"
invoice_total="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT total FROM billing.invoices WHERE partner_id='$paid_id' AND billing_model='PLAN' AND charge_type='PLAN_MONTHLY' AND service_period_start='$JAN1'::date ORDER BY created_at DESC LIMIT 1;")"
test "$invoice_total" = "630.00"
echo ok

printf 'commercial decisions and Charity approval are audit persisted... '
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.partner_commercial_mode_history WHERE partner_id='$charity_id' AND new_charity_status='APPROVED';")" -ge "1"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM billing.billing_events WHERE partner_id='$charity_id' AND event_type='CHARITY_APPROVED';")" -ge "1"
echo ok

echo 'HIMATE START-23.11.3k Commercial Status, Charity & Package Administration smoke passed'
