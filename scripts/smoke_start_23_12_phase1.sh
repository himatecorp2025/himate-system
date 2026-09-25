#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-2312-p1-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-2312-p1-partner.txt"
ADMIN_COOKIE="$TMP_ROOT/himate-2312-p1-admin.txt"
BODY="$TMP_ROOT/himate-2312-p1-body.json"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$ADMIN_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE" "$ADMIN_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
PARTNER_EMAIL="ci-2312-p1-partner-$STAMP@example.com"
PARTNER_PASSWORD="$(python3 -c 'import secrets; print("P1!"+secrets.token_urlsafe(24))')"
ADMIN_EMAIL="ci-2312-p1-admin-$STAMP@example.com"
ADMIN_PASSWORD="$(python3 -c 'import secrets; print("A1!"+secrets.token_urlsafe(24))')"
ROLE_KEY="ci_2312_p1_$STAMP"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

login_payload() {
  python3 - "$1" "$2" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
}

curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$(login_payload "$OWNER_EMAIL" "$OWNER_PASSWORD")" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'verify Starter/Business/Premium production package contracts... '
starter="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/plans/STARTER")"
business="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/plans/BUSINESS")"
flex="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/plans/FLEX")"
printf '%s\n%s\n%s\n' "$starter" "$business" "$flex" | python3 -c '
import json,sys
docs=[json.loads(x) for x in sys.stdin if x.strip()]
by={d["plan_key"]:d for d in docs}
assert by["STARTER"]["module_limit"]==10,by["STARTER"]
assert len(by["STARTER"]["fixed_module_keys"])==10,by["STARTER"]
assert by["STARTER"]["ready"] is True,by["STARTER"]
assert by["BUSINESS"]["module_limit"]==20,by["BUSINESS"]
assert len(by["BUSINESS"]["fixed_module_keys"])==20,by["BUSINESS"]
assert by["BUSINESS"]["ready"] is True,by["BUSINESS"]
assert by["FLEX"]["module_limit"] is None,by["FLEX"]
assert by["FLEX"]["selection_mode"]=="UNLIMITED" and by["FLEX"]["display_name"]=="Premium",by["FLEX"]'
echo ok

printf 'create normal Business partner and Portal owner... '
partner="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d '{"display_name":"START 23.12 Phase 1 Partner","legal_name":"START 23.12 Phase 1 Partner LLC","brand_name":"Phase 1 Partner","contact_name":"Portal Owner","contact_email":"portal-owner@example.com","country":"US"}' "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
portal_payload="$(python3 - "$PARTNER_EMAIL" "$PARTNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Phase 1 Portal Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$portal_payload" "$BASE_URL/api/v1/partners/$partner_id/portal-users" >/dev/null
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$(login_payload "$PARTNER_EMAIL" "$PARTNER_PASSWORD")" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
echo ok

printf 'activate Business plan for entitlement acceptance... '
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"currency":"USD","required_amount":0,"waived":true,"waiver_reason":"START-23.12 Phase 1 acceptance"}' "$BASE_URL/api/v1/billing/partners/$partner_id/license" >/dev/null
payment_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({"provider_customer_id":"cus_2312p1_"+s,"payment_method_id":"pm_2312p1_"+s,"autopay_enabled":True}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$payment_payload" "$BASE_URL/api/v1/payments/partners/$partner_id/profile" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"plan_key":"BUSINESS","billing_frequency":"MONTHLY","reason":"START-23.12 Phase 1 runtime authorization"}' "$BASE_URL/api/v1/billing/partners/$partner_id/plan" >/dev/null
marketplace="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/modules")"
printf '%s' "$marketplace" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[m for m in d["items"] if m.get("marketplace_visible") is True]; assert len(xs)>=1,xs; active=[m for m in xs if m["access_state"]=="ACTIVE"]; locked=[m for m in xs if m["access_state"]=="LOCKED"]; coming=[m for m in xs if m["access_state"]=="COMING_SOON"]; assert len(active)==20,active; assert len(active)+len(locked)+len(coming)==len(xs),(len(active),len(locked),len(coming),len(xs)); assert {"needs_assessment","two_factor_authentication"}.issubset({m["key"] for m in coming}),coming'
active_key="$(printf '%s' "$marketplace" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(next(m["key"] for m in d["items"] if m.get("access_state")=="ACTIVE" and m.get("executable") is True))')"
second_active_key="$(printf '%s' "$marketplace" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=[m["key"] for m in d["items"] if m.get("access_state")=="ACTIVE" and m.get("executable") is True]; print(xs[1])')"
locked_key="$(printf '%s' "$marketplace" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(next(m["key"] for m in d["items"] if m.get("access_state")=="LOCKED"))')"
echo ok

printf 'locked organization module is rejected by backend runtime guard... '
code="$(status "$PARTNER_COOKIE" GET "/partner/api/v1/runtime/modules/$locked_key/access")"
test "$code" = "403"
grep -q 'MODULE_NOT_OWNED' "$BODY"
echo ok

printf 'owned module is granted by backend runtime guard... '
curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/runtime/modules/$active_key/access" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["access_state"]=="GRANTED"; assert d["security_rule"]=="PARTNER_ENTITLEMENT_INTERSECT_USER_ASSIGNMENT"'
echo ok

printf 'user-specific module assignment is enforced server-side... '
me="$(curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/auth/me")"
user_id="$(printf '%s' "$me" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
selection="$(python3 - "$active_key" <<'PY'
import json,sys
print(json.dumps({"access_mode":"SELECTED","module_keys":[sys.argv[1]]}))
PY
)"
curl -fsS -b "$PARTNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$selection" "$BASE_URL/partner/api/v1/users/$user_id/modules" >/dev/null
curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/runtime/modules/$active_key/access" >/dev/null
code="$(status "$PARTNER_COOKIE" GET "/partner/api/v1/runtime/modules/$second_active_key/access")"
test "$code" = "403"
grep -q 'MODULE_NOT_ASSIGNED' "$BODY"
curl -fsS -b "$PARTNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d '{"access_mode":"ALL_OWNED","module_keys":[]}' "$BASE_URL/partner/api/v1/users/$user_id/modules" >/dev/null
echo ok

printf 'non-owner control-plane admin cannot activate Golden Test Partner mode... '
role_payload="$(python3 - "$ROLE_KEY" <<'PY'
import json,sys
print(json.dumps({
 "key":sys.argv[1],
 "label_en":"START 23.12 Phase 1 Operator",
 "label_hu":"START 23.12 Phase 1 Operátor",
 "description_en":"Phase 1 privilege-boundary acceptance",
 "description_hu":"Phase 1 jogosultsági határ elfogadás",
 "permissions":["partners.read","partners.write"]
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$role_payload" "$BASE_URL/api/v1/admin/roles" >/dev/null
admin_payload="$(python3 - "$ADMIN_EMAIL" "$ADMIN_PASSWORD" "$ROLE_KEY" <<'PY'
import json,sys
print(json.dumps({"name":"Phase 1 Operator","email":sys.argv[1],"password":sys.argv[2],"roles":[sys.argv[3]]}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$admin_payload" "$BASE_URL/api/v1/admin/users" >/dev/null
curl -fsS -c "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$(login_payload "$ADMIN_EMAIL" "$ADMIN_PASSWORD")" "$BASE_URL/api/v1/auth/login" >/dev/null
code="$(status "$ADMIN_COOKIE" PATCH "/api/v1/partners/$partner_id" -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"unauthorized Golden Test activation"}')"
test "$code" = "403"
grep -q 'OWNER_REQUIRED' "$BODY"
echo ok

echo 'HIMATE START-23.12 Phase 1 runtime acceptance smoke passed'
