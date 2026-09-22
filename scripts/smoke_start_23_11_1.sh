#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23111-owner.txt"
PORTAL_COOKIE="$TMP_ROOT/himate-start23111-portal.txt"
BODY="$TMP_ROOT/himate-start23111-body.json"
rm -f "$OWNER_COOKIE" "$PORTAL_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$PORTAL_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
MODULE_KEY="ci.start23111_$STAMP"
UNCONFIGURED_KEY="ci.start23111_unconfigured_$STAMP"
PORTAL_EMAIL="start23111-$STAMP@example.com"
PORTAL_PASSWORD="Strong-Start23111!$STAMP"
QUOTE_A="Q-23111-A-$STAMP"
QUOTE_B="Q-23111-B-$STAMP"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'START-23.11.1 owner login... '
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'canonical 38-module registry is normalized into four primary groups... '
modules="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
groups="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/module-groups")"
python3 - "$modules" "$groups" <<'PY'
import json,sys
mods=json.loads(sys.argv[1])["items"]
groups=json.loads(sys.argv[2])["items"]
legacy=[m for m in mods if m.get("system") is True and m.get("legacy_reference")=="KLAVIERHAUS_LEGACY"]
assert len(legacy)==38,len(legacy)
counts={}
for m in legacy: counts[m["group_key"]]=counts.get(m["group_key"],0)+1
assert counts=={"finance_invoicing":3,"technical":16,"marketing":8,"website_events":11},counts
assert all(m["implementation_state"]=="LEGACY_REFERENCE" for m in legacy),legacy
primary={g["group_key"] for g in groups if g.get("is_primary_navigation") is True}
assert primary=={"finance_invoicing","technical","marketing","website_events"},primary
assert next(m for m in legacy if m["key"]=="workshop_workflow")["group_key"]=="technical"
PY
echo ok

printf 'create two isolated partners and an unpublished development module... '
partner_a="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.11.1 Partner A","legal_name":"START 23.11.1 Partner A LLC","brand_name":"23111A","contact_name":"Owner A","contact_email":"23111-a@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_b="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.11.1 Partner B","legal_name":"START 23.11.1 Partner B LLC","brand_name":"23111B","contact_name":"Owner B","contact_email":"23111-b@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_a_id="$(printf '%s' "$partner_a" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
partner_b_id="$(printf '%s' "$partner_b" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
module_payload="$(python3 - "$MODULE_KEY" <<'PY'
import json,sys
print(json.dumps({
 "key":sys.argv[1],"group_key":"technical","label_en":"START 23.11.1 Contract Module","label_hu":"START 23.11.1 Szerződéses Modul",
 "description_en":"Individual contract data-model acceptance","description_hu":"Egyedi szerződéses adatmodell elfogadás",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0",
 "default_monthly_price":999,"default_activation_fee":9999,
 "availability":"ACTIVE","module_type":"FEATURE","owner_team":"Platform",
 "manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$module_payload" "$BASE_URL/api/v1/modules" >/dev/null
created="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
printf '%s' "$created" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["publication_status"]=="UNPUBLISHED",m; assert m["implementation_state"]=="IN_DEVELOPMENT",m; assert m["pricing_authority"]=="PARTNER_CONTRACT",m' "$MODULE_KEY"
echo ok

printf 'PUBLISHED is fail-closed until implementation is READY... '
code="$(status "$OWNER_COOKIE" PATCH "/api/v1/modules/$MODULE_KEY" -H 'Content-Type: application/json' -d '{"publication_status":"PUBLISHED"}')"
test "$code" = "409"
grep -q 'MODULE_NOT_READY' "$BODY"
echo ok

printf 'individual partner terms accept negotiated activation fee and enforce USD 1500 minimum commitment... '
code="$(status "$OWNER_COOKIE" PUT "/api/v1/billing/partners/$partner_a_id/terms" -H 'Content-Type: application/json' -d '{"minimum_monthly_commitment":1499,"reason":"START-23.11.1 minimum guard"}')"
test "$code" = "400"
grep -q 'MINIMUM_MONTHLY_COMMITMENT' "$BODY"
today="$(python3 - <<'PY'
from datetime import datetime,timezone
print(datetime.now(timezone.utc).date().isoformat())
PY
)"
terms_payload="$(python3 - "$today" "$QUOTE_A" <<'PY'
import json,sys
print(json.dumps({
 "currency":"USD","activation_fee":4200,"activation_fee_waived":False,
 "base_monthly_fee":850,"minimum_monthly_commitment":1750,"quote_reference":sys.argv[2],
 "annual_increase_percent":0,"price_effective_from":sys.argv[1],"service_anchor_date":sys.argv[1],
 "reason":"START-23.11.1 negotiated quote"
}))
PY
)"
terms_a="$(curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$terms_payload" "$BASE_URL/api/v1/billing/partners/$partner_a_id/terms")"
printf '%s' "$terms_a" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["activation_fee"]==4200,d; assert d["base_monthly_fee"]==850,d; assert d["minimum_monthly_commitment"]==1750,d; assert d["quote_reference"]==sys.argv[1],d; assert d["pricing_model"]=="INDIVIDUAL_QUOTE",d; assert d["commercial_configured"] is True,d; assert d["terms_version"]>=2,d' "$QUOTE_A"
history="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_a_id/terms-history")"
printf '%s' "$history" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["count"]>=1,d; x=d["items"][0]; assert x["quote_reference"]==sys.argv[1],x; assert x["activation_fee"]==4200,x; assert x["minimum_monthly_commitment"]==1750,x' "$QUOTE_A"
license="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_a_id/license")"
printf '%s' "$license" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["required_amount"]==4200,d'
if docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE billing.partner_terms_history SET quote_reference='TAMPERED' WHERE partner_id='$partner_a_id';" >/dev/null 2>&1; then
  echo "commercial history mutation unexpectedly succeeded" >&2
  exit 1
fi
echo ok

printf 'partner-module contract price is explicit and independent from module reference price... '
commercial_a="$(python3 - "$QUOTE_A" <<'PY'
import json,sys
print(json.dumps({"visible":True,"included_in_base":False,"contract_currency":"USD","quote_reference":sys.argv[1],"partner_price":275,"partner_activation_fee":600,"reason":"START-23.11.1 Partner A quote"}))
PY
)"
commercial_b="$(python3 - "$QUOTE_B" <<'PY'
import json,sys
print(json.dumps({"visible":True,"included_in_base":False,"contract_currency":"USD","quote_reference":sys.argv[1],"partner_price":910,"partner_activation_fee":1200,"reason":"START-23.11.1 Partner B quote"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$commercial_a" "$BASE_URL/api/v1/partners/$partner_a_id/modules/$MODULE_KEY" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$commercial_b" "$BASE_URL/api/v1/partners/$partner_b_id/modules/$MODULE_KEY" >/dev/null
encoded="$partner_a_id%2C$partner_b_id"
matrix="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/module-commercial-matrix?partner_ids=$encoded")"
printf '%s' "$matrix" | python3 -c 'import json,sys; d=json.load(sys.stdin); key,a,b,qa,qb=sys.argv[1:]; rows={(x["partner_id"],x["key"]):x for x in d["items"]}; xa=rows[(a,key)]; xb=rows[(b,key)]; assert xa["partner_price"]==275 and xb["partner_price"]==910,(xa,xb); assert xa["quote_reference"]==qa and xb["quote_reference"]==qb,(xa,xb); assert xa["commercial_configured"] is True and xb["commercial_configured"] is True; assert xa["price_source"]!="MODULE_REFERENCE_ONLY" and xb["price_source"]!="MODULE_REFERENCE_ONLY"' "$MODULE_KEY" "$partner_a_id" "$partner_b_id" "$QUOTE_A" "$QUOTE_B"
echo ok

printf 'unpublished module is invisible and non-activatable in Partner Portal... '
portal_payload="$(python3 - "$PORTAL_EMAIL" "$PORTAL_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 23.11.1 Portal Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_payload" "$BASE_URL/api/v1/partners/$partner_a_id/portal-users" >/dev/null
portal_login="$(python3 - "$PORTAL_EMAIL" "$PORTAL_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$PORTAL_COOKIE" -H 'Content-Type: application/json' -d "$portal_login" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
portal_before="$(curl -fsS -b "$PORTAL_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$portal_before" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert sys.argv[1] not in {x["key"] for x in d["items"]},d' "$MODULE_KEY"
code="$(status "$PORTAL_COOKIE" POST "/partner/api/v1/modules/$MODULE_KEY/activate" -H 'Content-Type: application/json' -d '{}')"
test "$code" = "409"
grep -q 'MODULE_UNPUBLISHED' "$BODY"
echo ok

printf 'READY then PUBLISHED exposes the contracted module without changing its partner price... '
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"implementation_state":"READY"}' "$BASE_URL/api/v1/modules/$MODULE_KEY" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"publication_status":"PUBLISHED"}' "$BASE_URL/api/v1/modules/$MODULE_KEY" >/dev/null
portal_after="$(curl -fsS -b "$PORTAL_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$portal_after" | python3 -c 'import json,sys; d=json.load(sys.stdin); key,quote=sys.argv[1:]; m=next(x for x in d["items"] if x["key"]==key); assert m["partner_price"]==275,m; assert m["commercial_configured"] is True,m; assert m["commercial_ready"] is True,m; assert m["quote_reference"]==quote,m; assert m["entitlement_state"]=="INACTIVE",m' "$MODULE_KEY" "$QUOTE_A"
echo ok

printf 'published module without partner-specific commercial configuration fails closed... '
unconfigured_payload="$(python3 - "$UNCONFIGURED_KEY" <<'PY'
import json,sys
print(json.dumps({
 "key":sys.argv[1],"group_key":"technical","label_en":"START 23.11.1 Unconfigured","label_hu":"START 23.11.1 Nincs Arazva",
 "currency":"USD","version":"1.0.0","latest_version":"1.0.0",
 "default_monthly_price":777,"default_activation_fee":777,
 "availability":"ACTIVE","publication_status":"PUBLISHED","implementation_state":"READY",
 "module_type":"FEATURE","owner_team":"Platform","manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$unconfigured_payload" "$BASE_URL/api/v1/modules" >/dev/null
unconfigured_portal="$(curl -fsS -b "$PORTAL_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$unconfigured_portal" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["commercial_configured"] is False,m; assert m["commercial_ready"] is False,m; assert m["can_activate"] is False,m; assert m["partner_price"]==0,m' "$UNCONFIGURED_KEY"
# price-at is intentionally an internal Catalog/Billing contract, not a public control-plane route.
code="$(status "$OWNER_COOKIE" GET "/api/v1/partners/$partner_a_id/modules/$UNCONFIGURED_KEY/price-at?at=$today")"
test "$code" = "405"
grep -q 'METHOD' "$BODY"
code="$(status "$PORTAL_COOKIE" POST "/partner/api/v1/modules/$UNCONFIGURED_KEY/activate" -H 'Content-Type: application/json' -d '{}')"
test "$code" = "409"
grep -q 'COMMERCIAL_TERMS_REQUIRED' "$BODY"
echo ok

printf 'explicit base-package inclusion is commercial-ready with zero extra module fee... '
base_payload="$(python3 - "$QUOTE_A" <<'PY'
import json,sys
print(json.dumps({
 "visible":True,
 "included_in_base":True,
 "contract_currency":"USD",
 "quote_reference":sys.argv[1]+"-BASE",
 "reason":"START-23.11.1 base package inclusion"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$base_payload" "$BASE_URL/api/v1/partners/$partner_a_id/modules/$UNCONFIGURED_KEY" >/dev/null
base_price="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_a_id/modules/$UNCONFIGURED_KEY/price-at?at=$today")"
printf '%s' "$base_price" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["included_in_base"] is True,d; assert d["price"]==0,d'
base_portal="$(curl -fsS -b "$PORTAL_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$base_portal" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["included_in_base"] is True,m; assert m["commercial_ready"] is True,m; assert m["partner_price"]==0,m; assert m["can_activate"] is True,m' "$UNCONFIGURED_KEY"
echo ok

printf 'contracted module activation and Billing cancellation state stay synchronized... '
activated="$(curl -fsS -b "$PORTAL_COOKIE" -X POST -H 'Content-Type: application/json' -d '{}' "$BASE_URL/partner/api/v1/modules/$MODULE_KEY/activate")"
printf '%s' "$activated" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=d["module"]; assert m["status"]=="ACTIVE",m; assert m["entitlement_state"]=="ACTIVE",m'
curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/partners/$partner_a_id/summary" >/dev/null
cancelled="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"cancel_at_period_end":true,"reason":"START-23.11.1 lifecycle sync"}' "$BASE_URL/api/v1/billing/partners/$partner_a_id/subscriptions/$MODULE_KEY")"
printf '%s' "$cancelled" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle_state"]=="CANCEL_PENDING",d'
catalog_state="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_a_id/modules")"
printf '%s' "$catalog_state" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["status"]=="ACTIVE",m; assert m["entitlement_state"]=="CANCEL_PENDING",m' "$MODULE_KEY"
withdrawn="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"cancel_at_period_end":false,"reason":"START-23.11.1 lifecycle withdrawal"}' "$BASE_URL/api/v1/billing/partners/$partner_a_id/subscriptions/$MODULE_KEY")"
printf '%s' "$withdrawn" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle_state"]=="ACTIVE",d'
catalog_state="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/partners/$partner_a_id/modules")"
printf '%s' "$catalog_state" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["entitlement_state"]=="ACTIVE",m' "$MODULE_KEY"
echo ok

echo "HIMATE START-23.11.1 module registry and individual commercial model smoke passed"
