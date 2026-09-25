#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-start23113-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-start23113-partner.txt"
BODY="$TMP_ROOT/himate-start23113-body.json"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
PARTNER_EMAIL="ci-start23113-partner-$STAMP@example.com"
PARTNER_PASSWORD="$(python3 -c 'import secrets; print("Pm3!"+secrets.token_urlsafe(24))')"
TODAY="$(date -u +%Y-%m-%d)"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'create Marketplace acceptance partner and Partner Portal owner... '
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.11.3 Marketplace Partner","legal_name":"START 23.11.3 Marketplace Partner LLC","brand_name":"Marketplace Partner","contact_name":"Marketplace Owner","contact_email":"marketplace-owner@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
portal_payload="$(python3 - "$PARTNER_EMAIL" "$PARTNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Marketplace Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_payload" "$BASE_URL/api/v1/partners/$partner_id/portal-users" >/dev/null
portal_login="$(python3 - "$PARTNER_EMAIL" "$PARTNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_login" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
echo ok

printf 'Central-4 exposes 40 canonical Marketplace modules before live release without making planned modules executable... '
before="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$before" | python3 -c 'import json,sys; d=json.load(sys.stdin); canonical=[m for m in d["items"] if m.get("marketplace_visible") is True]; assert len(canonical)==40,len(canonical); assert all(m.get("marketplace_summary","").strip() for m in canonical); assert all(m["access_state"]=="COMING_SOON" for m in canonical),canonical; assert all(m["executable"] is False for m in canonical),canonical; assert all(m["can_activate"] is False for m in canonical),canonical; planned=[m for m in canonical if m["key"] in {"needs_assessment","two_factor_authentication"}]; assert len(planned)==2,planned; assert all(m["implementation_state"]=="IN_DEVELOPMENT" and m["publication_status"]=="UNPUBLISHED" for m in planned),planned; assert d["marketplace_model"]=="DISCOVERY_SEPARATE_FROM_EXECUTION",d'
echo ok

printf 'publish the canonical Marketplace portfolio for plan-entitlement acceptance... '
catalog="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
CANONICAL_KEYS="$(printf '%s' "$catalog" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(" ".join(m["key"] for m in d["items"] if m.get("system") is True and m.get("legacy_reference")=="KLAVIERHAUS_LEGACY"))')"
test "$(printf '%s
' "$CANONICAL_KEYS" | wc -w | tr -d ' ')" = "38"
for key in $CANONICAL_KEYS; do
  curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"implementation_state":"READY"}' "$BASE_URL/api/v1/modules/$key" >/dev/null
  curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"publication_status":"PUBLISHED"}' "$BASE_URL/api/v1/modules/$key" >/dev/null
done
echo ok

STARTER_KEYS="$(printf '%s' "$catalog" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[m["key"] for m in d["items"] if m.get("system") is True and m.get("legacy_reference")=="KLAVIERHAUS_LEGACY"]; print(json.dumps(xs[:3]))')"
BUSINESS_KEYS="$(printf '%s' "$catalog" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[m["key"] for m in d["items"] if m.get("system") is True and m.get("legacy_reference")=="KLAVIERHAUS_LEGACY"]; print(json.dumps(xs[:10]))')"

printf 'configure Starter and Business from canonical Marketplace modules... '
starter_payload="$(python3 - "$STARTER_KEYS" "$TODAY" <<'PY'
import json,sys
print(json.dumps({"fixed_module_keys":json.loads(sys.argv[1]),"effective_at":sys.argv[2]}))
PY
)"
business_payload="$(python3 - "$BUSINESS_KEYS" "$TODAY" <<'PY'
import json,sys
print(json.dumps({"fixed_module_keys":json.loads(sys.argv[1]),"effective_at":sys.argv[2]}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$starter_payload" "$BASE_URL/api/v1/billing/plans/STARTER" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d "$business_payload" "$BASE_URL/api/v1/billing/plans/BUSINESS" >/dev/null
echo ok

printf 'activate Business with autopay-ready payment profile... '
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":0,"waived":true,"waiver_reason":"START-23.11.3 marketplace acceptance"}' "$BASE_URL/api/v1/billing/partners/$partner_id/license" >/dev/null
payment_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({"provider_customer_id":"cus_start23113_"+s,"payment_method_id":"pm_start23113_"+s,"autopay_enabled":True}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$payment_payload" "$BASE_URL/api/v1/payments/partners/$partner_id/profile" >/dev/null
business="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"plan_key":"BUSINESS","billing_frequency":"MONTHLY","reason":"START-23.11.3 Marketplace Business acceptance"}' "$BASE_URL/api/v1/billing/partners/$partner_id/plan")"
printf '%s' "$business" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["plan_key"]=="BUSINESS",d; assert len(d["active_module_keys"])==10,d'
echo ok

printf 'Business Marketplace exposes 10 included modules, 28 locked Flex-upgrade modules and 2 planned modules... '
marketplace="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$marketplace" | python3 -c 'import json,sys; d=json.load(sys.stdin); canonical=[m for m in d["items"] if m.get("marketplace_visible") is True]; assert len(canonical)==40,len(canonical); active=[m for m in canonical if m["access_state"]=="ACTIVE"]; locked=[m for m in canonical if m["access_state"]=="LOCKED"]; coming=[m for m in canonical if m["access_state"]=="COMING_SOON"]; assert len(active)==10,len(active); assert len(locked)==28,len(locked); assert len(coming)==2,coming; assert {m["key"] for m in coming}=={"needs_assessment","two_factor_authentication"},coming; assert all(m["executable"] is True for m in active+locked),active+locked; assert all(m["executable"] is False for m in coming),coming; assert all(m.get("in_current_plan") is True for m in active),active; assert all("FLEX" in m.get("upgrade_plan_keys",[]) for m in locked),locked; assert all(m.get("recommended_upgrade_plan")=="FLEX" for m in locked),locked; assert all(m.get("marketplace_summary","").strip() for m in canonical); assert d.get("current_plan_key")=="BUSINESS",d'
echo ok

printf 'dashboard uses the same enriched 40-module Marketplace read model... '
dashboard="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/dashboard")"
printf '%s' "$dashboard" | python3 -c 'import json,sys; d=json.load(sys.stdin); mods=d["modules"]["items"]; canonical=[m for m in mods if m.get("marketplace_visible") is True]; assert len(canonical)==40,len(canonical); assert sum(1 for m in canonical if m["access_state"]=="ACTIVE")==10; assert sum(1 for m in canonical if m["access_state"]=="LOCKED")==28; assert sum(1 for m in canonical if m["access_state"]=="COMING_SOON")==2'
echo ok

printf 'locked managed-plan module cannot bypass plan entitlement through direct activation... '
locked_key="$(printf '%s' "$marketplace" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(next(m["key"] for m in d["items"] if m.get("marketplace_visible") is True and m["access_state"]=="LOCKED"))')"
code="$(status "$PARTNER_COOKIE" POST "/partner/api/v1/modules/$locked_key/activate" -H 'Content-Type: application/json' -d '{}')"
test "$code" = "409"
grep -q 'PLAN_MANAGED_MODULES' "$BODY"
echo ok

echo 'HIMATE START-23.11.3 Module Marketplace smoke passed'
