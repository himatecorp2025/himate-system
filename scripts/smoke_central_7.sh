#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-central7-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-central7-partner.txt"
LANDING="$TMP_ROOT/himate-central7-landing.html"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$LANDING"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$LANDING"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
YEAR="$(date -u +%Y)"

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'CENTRAL-7 public entry points preserve Partner Portal + Contact without advertising Central Admin... '
curl -fsS "$BASE_URL/" -o "$LANDING"
grep -q 'href="/partner/login"' "$LANDING"
grep -q 'href="/contact"' "$LANDING"
if grep -q 'href="/login"' "$LANDING"; then
  echo "public landing still advertises Central Admin login" >&2
  exit 1
fi
echo ok

printf 'CENTRAL-7 Dashboard keeps weekly/monthly Impact and authoritative Recent Activity... '
dashboard="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
printf '%s' "$dashboard" | python3 -c '
import json,sys
d=json.load(sys.stdin)
impact=d["impact"]; activity=d["activity"]
assert impact["source"]=="IMPACT_METRIC_VALUES",impact
assert len(impact["trend"])==12,len(impact["trend"])
assert 52 <= len(impact["weekly_trend"]) <= 54,len(impact["weekly_trend"])
assert activity["source"]=="IDENTITY_APPEND_ONLY_AUDIT",activity
assert activity["count"]==len(activity["items"]) and activity["count"]>=1,activity
assert all(x.get("outcome")=="SUCCESS" for x in activity["items"]),activity
'
echo ok

printf 'CENTRAL-7 Test/Golden Partner remains visible with bilingual category data... '
golden="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners?limit=200&offset=0&core_only=true&q=Golden%20Test%20Partner")"
printf '%s' "$golden" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert d["count"]>=1,d
items=[x for x in d["items"] if x.get("test_partner") is True]
assert items,d
assert all(x.get("category_name_en") and x.get("category_name_hu") for x in items),items
'
echo ok

printf 'CENTRAL-7 Module Registry categories, unique keys and package module references remain coherent... '
modules="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
groups="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/module-groups")"
plans="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/plans")"
python3 - "$modules" "$groups" "$plans" <<'PY'
import json,sys
mods=json.loads(sys.argv[1])["items"]
groups=json.loads(sys.argv[2])["items"]
plans=json.loads(sys.argv[3])["items"]
assert mods and groups and plans
keys=[x["key"] for x in mods]
assert len(keys)==len(set(keys)),keys
group_keys={x["group_key"] for x in groups}
assert all(x.get("group_key") in group_keys for x in mods),[(x.get("key"),x.get("group_key")) for x in mods if x.get("group_key") not in group_keys]
by={x["plan_key"]:x for x in plans}
assert {"STARTER","BUSINESS","FLEX"} <= set(by),by.keys()
starter,business,premium=by["STARTER"],by["BUSINESS"],by["FLEX"]
assert starter["selection_mode"]=="FIXED" and starter["module_limit"]==10,starter
assert business["selection_mode"]=="FIXED" and business["module_limit"]==20,business
assert premium["selection_mode"]=="UNLIMITED" and premium["module_limit"] is None and premium["unlimited_modules"] is True,premium
module_keys=set(keys)
assert set(starter.get("fixed_module_keys",[])) <= module_keys,starter
assert set(business.get("fixed_module_keys",[])) <= module_keys,business
assert len(starter.get("fixed_module_keys",[]))==10,starter
assert len(business.get("fixed_module_keys",[]))==20,business
PY
echo ok

printf 'CENTRAL-7 Premium acceptance partner still receives every eligible released module... '
premium_partners="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners?limit=100&offset=0&q=Central-5%20Premium%20Partner")"
PREMIUM_ID="$(printf '%s' "$premium_partners" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[x for x in d["items"] if x.get("display_name")=="Central-5 Premium Partner"]; assert xs,d; print(xs[-1]["id"])')"
premium_modules="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$PREMIUM_ID/modules")"
python3 - "$modules" "$premium_modules" <<'PY'
import json,sys
mods=json.loads(sys.argv[1])["items"]
state=json.loads(sys.argv[2])["items"]
eligible={x["key"] for x in mods if x.get("publication_status")=="PUBLISHED" and x.get("implementation_state")=="READY" and x.get("availability")=="ACTIVE"}
active={x["key"] for x in state if x.get("status")=="ACTIVE" and x.get("plan_key")=="FLEX"}
assert active==eligible,(len(active),len(eligible),sorted(eligible-active)[:10],sorted(active-eligible)[:10])
PY
echo ok

printf 'CENTRAL-7 paid onboarding remains ACTIVE, Portal-enabled and MFA-optional after finance closure... '
paid_partners="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners?limit=100&offset=0&q=Central-6%20Paid%20Partner")"
paid_meta="$(printf '%s' "$paid_partners" | python3 -c '
import json,sys
d=json.load(sys.stdin); xs=[x for x in d["items"] if str(x.get("display_name","")).startswith("Central-6 Paid Partner ")]
assert xs,d
x=xs[-1]
stamp=x["display_name"].rsplit(" ",1)[-1]
assert stamp.isdigit(),x
print(x["id"]+"|"+stamp)
')"
PAID_ID="${paid_meta%%|*}"
PAID_STAMP="${paid_meta#*|}"
paid_onboarding="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$PAID_ID/onboarding")"
printf '%s' "$paid_onboarding" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="ACTIVE" and d["portal_enabled"] is True,d'
partner_login="$(python3 - "$PAID_STAMP" <<'PY'
import json,sys
print(json.dumps({"email":"central6.paid."+sys.argv[1]+"@himate.test","password":"Central6Paid!2345Aa","remember":False}))
PY
)"
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$partner_login" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
echo ok

printf 'CENTRAL-7 manual invoice preserves payment provider, deadline and Partner Portal visibility... '
paid_invoices="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/invoices?partner_id=$PAID_ID")"
INVOICE_ID="$(printf '%s' "$paid_invoices" | python3 -c '
import json,sys
d=json.load(sys.stdin); xs=[x for x in d["items"] if x.get("workflow_status")=="PAID" and x.get("provider")=="MANUAL"]
assert xs,d
x=xs[-1]
assert x.get("payment_deadline_at"),x
assert x.get("provider_payment_id")=="C6-MANUAL-PAID",x
print(x["id"])
')"
portal_invoices="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/billing/invoices")"
printf '%s' "$portal_invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); iid=sys.argv[1]; x=next(v for v in d["items"] if v["id"]==iid); assert x["workflow_status"]=="PAID" and x["provider"]=="MANUAL",x' "$INVOICE_ID"
echo ok

printf 'CENTRAL-7 sponsored onboarding remains ACTIVE with waiver evidence and no zero-dollar invoice... '
zero_partners="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners?limit=100&offset=0&q=Central-6%20Sponsored%20Partner")"
ZERO_ID="$(printf '%s' "$zero_partners" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[x for x in d["items"] if str(x.get("display_name","")).startswith("Central-6 Sponsored Partner ")]; assert xs,d; print(xs[-1]["id"])')"
zero_state="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$ZERO_ID/onboarding")"
printf '%s' "$zero_state" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="ACTIVE" and d["portal_enabled"] is True and d["support_waiver_documented"] is True,d'
zero_invoices="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/invoices?partner_id=$ZERO_ID")"
printf '%s' "$zero_invoices" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["count"]==0,d'
echo ok

printf 'CENTRAL-7 finance overview remains ledger-backed after the complete first-half sequence... '
overview="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/finance/overview")"
printf '%s' "$overview" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["source"]=="CENTRAL_6_FINANCE_LEDGER",d; assert any(x.get("paid",0)>=1 for x in d["currencies"]),d'
echo ok

echo 'CENTRAL-7 Central-1..6 runtime regression closure passed'
