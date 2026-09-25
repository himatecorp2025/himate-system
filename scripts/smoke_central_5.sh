#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-central5-owner.txt"
BODY="$TMP_ROOT/himate-central5-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
PREFIX="ci.central5.$STAMP"

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

printf 'Central-5 package contracts are 10 / 20 / Unlimited with net pricing... '
plans="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/plans")"
printf '%s' "$plans" | python3 -c '
import json,sys
d=json.load(sys.stdin); p={x["plan_key"]:x for x in d["items"]}
s,b,x=p["STARTER"],p["BUSINESS"],p["FLEX"]
assert s["monthly_price"]==990 and s["module_limit"]==10 and s["selection_mode"]=="FIXED",s
assert b["monthly_price"]==1490 and b["module_limit"]==20 and b["selection_mode"]=="FIXED",b
assert x["display_name"]=="Premium" and x["monthly_price"]==2490,x
assert x["module_limit"] is None and x["selection_mode"]=="UNLIMITED" and x["unlimited_modules"] is True,x
assert all(i["price_basis"]=="NET_PLUS_TAX" for i in (s,b,x)),p'
echo ok

printf 'create 21 released modules and configure HIMATE-defined Starter/Business sets... '
i=1
while [ "$i" -le 21 ]; do
  key="$PREFIX.$i"
  payload="$(python3 - "$key" "$i" <<'PY'
import json,sys
key,i=sys.argv[1:]
print(json.dumps({
 "key":key,"group_key":"client_operations","label_en":"Central-5 Module "+i,"label_hu":"Central-5 Modul "+i,
 "description_en":"Central-5 package acceptance","description_hu":"Central-5 csomag elfogadási teszt",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":0,"default_activation_fee":0,
 "availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY",
 "module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}
}))
PY
)"
  curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/modules" >/dev/null
  i=$((i+1))
done
starter_keys="$(python3 - "$PREFIX" <<'PY'
import json,sys
p=sys.argv[1]; print(json.dumps([f"{p}.{i}" for i in range(1,11)]))
PY
)"
business_keys="$(python3 - "$PREFIX" <<'PY'
import json,sys
p=sys.argv[1]; print(json.dumps([f"{p}.{i}" for i in range(1,21)]))
PY
)"
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$(python3 - "$starter_keys" <<'PY'
import json,sys; print(json.dumps({"fixed_module_keys":json.loads(sys.argv[1]),"reason":"Central-5 Starter 10"}))
PY
)" "$BASE_URL/api/v1/billing/plans/STARTER" >/dev/null
curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$(python3 - "$business_keys" <<'PY'
import json,sys; print(json.dumps({"fixed_module_keys":json.loads(sys.argv[1]),"reason":"Central-5 Business 20"}))
PY
)" "$BASE_URL/api/v1/billing/plans/BUSINESS" >/dev/null
echo ok

printf 'administrator VAT policy controls package gross quotes... '
profile="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/profile")"
vat_profile="$(printf '%s' "$profile" | python3 -c '
import json,sys
d=json.load(sys.stdin)
keys=["legal_name","registration_number","address","tax_id","contact_name","email","phone","bank_name","bank_address","account_number","iban","swift"]
out={k:d.get(k,"") for k in keys}; out.update({"vat_rate_percent":20,"vat_jurisdiction":"GB","tax_label":"VAT"})
print(json.dumps(out))')"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$vat_profile" "$BASE_URL/api/v1/billing/profile" >/dev/null
plans="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/plans")"
printf '%s' "$plans" | python3 -c '
import json,sys
d=json.load(sys.stdin); p={x["plan_key"]:x for x in d["items"]}
assert (p["STARTER"]["monthly_net_price"],p["STARTER"]["monthly_tax_amount"],p["STARTER"]["monthly_gross_price"])==(990,198,1188),p["STARTER"]
assert (p["BUSINESS"]["monthly_net_price"],p["BUSINESS"]["monthly_tax_amount"],p["BUSINESS"]["monthly_gross_price"])==(1490,298,1788),p["BUSINESS"]
assert (p["FLEX"]["monthly_net_price"],p["FLEX"]["monthly_tax_amount"],p["FLEX"]["monthly_gross_price"])==(2490,498,2988),p["FLEX"]'
echo ok

printf 'activate annual Premium and prove VAT-aware invoice plus Unlimited entitlement... '
partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"Central-5 Premium Partner","legal_name":"Central-5 Premium Partner LLC","brand_name":"Central5 Premium","contact_name":"Premium Owner","contact_email":"central5-premium@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":0,"waived":true,"waiver_reason":"Central-5 acceptance"}' "$BASE_URL/api/v1/billing/partners/$partner_id/license" >/dev/null
payment_payload="$(python3 - "$STAMP" <<'PY'
import json,sys;s=sys.argv[1]
print(json.dumps({"provider_customer_id":"cus_c5_"+s,"payment_method_id":"pm_c5_"+s,"autopay_enabled":True}))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$payment_payload" "$BASE_URL/api/v1/payments/partners/$partner_id/profile" >/dev/null
premium="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"plan_key":"FLEX","billing_frequency":"ANNUAL","reason":"Central-5 Premium Unlimited"}' "$BASE_URL/api/v1/billing/partners/$partner_id/plan")"
printf '%s' "$premium" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["display_name"]=="Premium" and d["selection_mode"]=="UNLIMITED" and d["module_limit"] is None,d; assert d["annual_net_price"]==22410 and d["annual_tax_amount"]==4482 and d["annual_gross_price"]==26892,d'
invoices="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_id/invoices")"
printf '%s' "$invoices" | python3 -c '
import json,sys
d=json.load(sys.stdin); x=next(i for i in d["items"] if i.get("charge_type")=="PLAN_ANNUAL_PREPAY")
assert x["net_total"]==22410 and x["tax_rate_percent"]==20 and x["tax_amount"]==4482 and x["gross_total"]==26892,x
assert [i["item_type"] for i in x["items"]]==["PLAN","TAX"],x'
eligible="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/modules" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for x in d["items"] if x.get("publication_status")=="PUBLISHED" and x.get("implementation_state")=="READY" and x.get("availability")=="ACTIVE"))')"
active="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for x in d["items"] if x.get("status")=="ACTIVE" and x.get("plan_key")=="FLEX"))')"
test "$active" = "$eligible"
echo ok

printf 'Premium cannot be replaced with a finite selection... '
code="$(status "$COOKIE" PUT "/api/v1/billing/partners/$partner_id/plan/modules" -H 'Content-Type: application/json' -d '{"module_keys":[],"reason":"must remain Unlimited"}')"
test "$code" = "409"
grep -q 'UNLIMITED_PLAN_MANAGED' "$BODY"
echo ok

printf 'a newly released module enters Premium automatically without package edit... '
future_key="$PREFIX.future"
future_payload="$(python3 - "$future_key" <<'PY'
import json,sys; key=sys.argv[1]
print(json.dumps({
 "key":key,"group_key":"client_operations","label_en":"Central-5 Future Module","label_hu":"Central-5 Jövőbeli Modul",
 "description_en":"Future Premium inclusion proof","description_hu":"Jövőbeli Premium jogosultság bizonyíték",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":0,"default_activation_fee":0,
 "availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY",
 "module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$future_payload" "$BASE_URL/api/v1/modules" >/dev/null
state="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$partner_id/modules")"
printf '%s' "$state" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["status"]=="ACTIVE" and m["entitlement_state"]=="ACTIVE" and m["plan_key"]=="FLEX",m' "$future_key"
echo ok

printf 'restore zero VAT baseline for later manual environments... '
zero_profile="$(printf '%s' "$profile" | python3 -c '
import json,sys
d=json.load(sys.stdin)
keys=["legal_name","registration_number","address","tax_id","contact_name","email","phone","bank_name","bank_address","account_number","iban","swift"]
out={k:d.get(k,"") for k in keys}; out.update({"vat_rate_percent":0,"vat_jurisdiction":d.get("vat_jurisdiction","GB"),"tax_label":d.get("tax_label","VAT")})
print(json.dumps(out))')"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$zero_profile" "$BASE_URL/api/v1/billing/profile" >/dev/null
echo ok

echo 'Central-5 Packages/pricing/VAT/Unlimited runtime acceptance passed'
