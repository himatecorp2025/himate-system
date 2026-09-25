#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-central4-owner.txt"
PARTNER_COOKIE="$TMP_ROOT/himate-central4-partner.txt"
rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE"
trap 'rm -f "$OWNER_COOKIE" "$PARTNER_COOKIE"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
GROUP_KEY="ci_central4_$STAMP"
MODULE_KEY="ci.central4_$STAMP"
PORTAL_EMAIL="central4-$STAMP@example.com"
PORTAL_PASSWORD="Strong-Central4!$STAMP"

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'Central-4 catalog accepts N >= 1 without fixed upper cardinality... '
BEFORE="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
BASE_COUNT="$(printf '%s' "$BEFORE" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=d["items"]; assert len(xs)>=1,xs; assert len({x["key"] for x in xs})==len(xs),xs; print(len(xs))')"
GROUPS="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/module-groups")"
printf '%s' "$GROUPS" | python3 -c 'import json,sys; d=json.load(sys.stdin); xs=d["items"]; assert len(xs)>=1,xs; required={"finance_invoicing","client_operations","marketing","website_events","security_system"}; assert required.issubset({x["group_key"] for x in xs}),(required,xs)'
echo ok

printf 'Central-4 can append another topic and module beyond the current catalog size... '
GROUP_PAYLOAD="$(python3 - "$GROUP_KEY" <<'PY'
import json,sys
k=sys.argv[1]
print(json.dumps({"group_key":k,"label_en":"Central 4 Dynamic Topic","label_hu":"Central 4 Dinamikus téma","sort_order":90}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$GROUP_PAYLOAD" "$BASE_URL/api/v1/module-groups" >/dev/null
MODULE_PAYLOAD="$(python3 - "$MODULE_KEY" "$GROUP_KEY" <<'PY'
import json,sys
key,group=sys.argv[1:]
print(json.dumps({
 "key":key,
 "group_key":group,
 "label_en":"Central 4 Dynamic Module",
 "label_hu":"Central 4 Dinamikus modul",
 "description_en":"Cardinality-independent Central-4 acceptance module.",
 "description_hu":"Darabszámtól független Central-4 elfogadási modul.",
 "currency":"USD",
 "version":"1.0.0",
 "latest_version":"1.0.0",
 "default_monthly_price":0,
 "default_activation_fee":0,
 "availability":"ACTIVE",
 "publication_status":"UNPUBLISHED",
 "implementation_state":"IN_DEVELOPMENT",
 "module_type":"FEATURE",
 "owner_team":"Platform",
 "manifest":{"schema_version":1}
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$MODULE_PAYLOAD" "$BASE_URL/api/v1/modules" >/dev/null
AFTER="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
printf '%s' "$AFTER" | python3 -c 'import json,sys; d=json.load(sys.stdin); base=int(sys.argv[1]); key=sys.argv[2]; xs=d["items"]; assert len(xs)==base+1,(len(xs),base); m=next(x for x in xs if x["key"]==key); assert m["implementation_state"]=="IN_DEVELOPMENT",m' "$BASE_COUNT" "$MODULE_KEY"
echo ok

printf 'Central-4 moves a module between topics through the authoritative group_key... '
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"group_key":"client_operations"}' "$BASE_URL/api/v1/modules/$MODULE_KEY" >/dev/null
MOVED="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
printf '%s' "$MOVED" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; m=next(x for x in d["items"] if x["key"]==key); assert m["group_key"]=="client_operations",m' "$MODULE_KEY"
echo ok

printf 'Central-4 planned Needs Assessment and optional 2FA modules remain present but unpublished... '
printf '%s' "$MOVED" | python3 -c 'import json,sys; d=json.load(sys.stdin); by={x["key"]:x for x in d["items"]}; required={"needs_assessment","two_factor_authentication"}; assert required.issubset(by),by.keys(); assert all(by[k]["implementation_state"]=="IN_DEVELOPMENT" and by[k]["publication_status"]=="UNPUBLISHED" for k in required),{k:by[k] for k in required}'
echo ok

printf 'Central-4 creates a Golden Test partner for runtime usage telemetry... '
PARTNER_PAYLOAD="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({
 "display_name":"Central 4 Usage Partner "+s,
 "legal_name":"Central 4 Usage Partner LLC "+s,
 "brand_name":"Central 4 Usage",
 "category_id":"cat_006",
 "lifecycle":"PROSPECT",
 "contact_name":"Central 4 Owner",
 "contact_email":"central4-owner-"+s+"@example.com",
 "registration_number":"CENTRAL4-"+s,
 "tax_id":"CENTRAL4-TAX-"+s,
 "country":"United States",
 "state_region":"New York",
 "city":"New York",
 "notes":"Central-4 runtime usage acceptance",
 "onboarding_request_id":"central4-"+s
}))
PY
)"
PARTNER="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$PARTNER_PAYLOAD" "$BASE_URL/api/v1/partners")"
PARTNER_ID="$(printf '%s' "$PARTNER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"Central-4 usage acceptance"}' "$BASE_URL/api/v1/partners/$PARTNER_ID" >/dev/null

curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"state":"PENDING_REVIEW","reason":"Central-4 Golden Test onboarding review"}'   "$BASE_URL/api/v1/billing/partners/$PARTNER_ID/onboarding" >/dev/null
CENTRAL4_CLASSIFY="$(python3 - "$STAMP" <<'PY'
import json,sys
stamp=sys.argv[1]
print(json.dumps({
 "state":"CLASSIFIED",
 "classification":"SPONSORED",
 "nominal_value":1,
 "currency":"USD",
 "evidence_reference":"CENTRAL-4-GOLDEN-"+stamp,
 "reason":"Central-4 Golden Test runtime telemetry support waiver"
}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d "$CENTRAL4_CLASSIFY"   "$BASE_URL/api/v1/billing/partners/$PARTNER_ID/onboarding" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"state":"ADMIN_APPROVAL","reason":"Central-4 Golden Test support evidence verified"}'   "$BASE_URL/api/v1/billing/partners/$PARTNER_ID/onboarding" >/dev/null
CENTRAL4_ACTIVE="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"state":"ACTIVE","reason":"Central-4 Golden Test final HIMATE approval"}'   "$BASE_URL/api/v1/billing/partners/$PARTNER_ID/onboarding")"
printf '%s' "$CENTRAL4_ACTIVE" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="ACTIVE" and d["portal_enabled"] is True,d'

PORTAL_PAYLOAD="$(python3 - "$PORTAL_EMAIL" "$PORTAL_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"Central 4 Portal Owner","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$PORTAL_PAYLOAD" "$BASE_URL/api/v1/partners/$PARTNER_ID/portal-users" >/dev/null
PORTAL_LOGIN="$(python3 - "$PORTAL_EMAIL" "$PORTAL_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$PARTNER_COOKIE" -H 'Content-Type: application/json' -d "$PORTAL_LOGIN" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
echo ok

printf 'Central-4 records successful module runtime usage without blocking the operation... '
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"implementation_state":"READY"}' "$BASE_URL/api/v1/modules/finance" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"publication_status":"PUBLISHED"}' "$BASE_URL/api/v1/modules/finance" >/dev/null
curl -fsS -b "$PARTNER_COOKIE" "$BASE_URL/partner/api/v1/runtime/modules/finance/access" >/dev/null
USAGE="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules/finance/usage")"
printf '%s' "$USAGE" | python3 -c 'import json,sys; d=json.load(sys.stdin); s=d["usage_summary"]; assert s["source"]=="CATALOG_RUNTIME_USAGE_EVENTS",s; assert s["events_total"]>=1,s; assert s["events_30d"]>=1,s; assert s["events_7d"]>=1,s; assert s["partners_with_usage"]>=1,s; assert any(x.get("usage_events_total",0)>=1 for x in d["items"]),d'
echo ok

echo "HIMATE Central-4 Modules runtime smoke passed"
