#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
EXPECTED_VERSION="${HIMATE_APP_VERSION:-0.8.29-start-23.11.4}"
TMP_ROOT="${TMPDIR:-/tmp}"
ADMIN_COOKIE="$TMP_ROOT/himate-start23114-admin.txt"
PARTNER_A_COOKIE="$TMP_ROOT/himate-start23114-a.txt"
PARTNER_B_COOKIE="$TMP_ROOT/himate-start23114-b.txt"
BODY="$TMP_ROOT/himate-start23114-body.json"
PNG="$TMP_ROOT/himate-start23114.png"
MEDIA_OUT="$TMP_ROOT/himate-start23114-media.png"
rm -f "$ADMIN_COOKIE" "$PARTNER_A_COOKIE" "$PARTNER_B_COOKIE" "$BODY" "$PNG" "$MEDIA_OUT"
trap 'rm -f "$ADMIN_COOKIE" "$PARTNER_A_COOKIE" "$PARTNER_B_COOKIE" "$BODY" "$PNG" "$MEDIA_OUT"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
PASSWORD_A="WspA4!${STAMP}Personalize"
PASSWORD_B="WspB4!${STAMP}Personalize"
EMAIL_A="workspace-a-${STAMP}@himate.test"
EMAIL_B="workspace-b-${STAMP}@himate.test"

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

admin_login="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$admin_login" "$BASE_URL/api/v1/auth/login" >/dev/null

printf '23.11.4 synchronized release... '
HEALTH="$(curl -fsS "$BASE_URL/api/v1/health")"
python3 - "$HEALTH" "$EXPECTED_VERSION" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); expected=sys.argv[2]
assert d["status"]=="ok" and d["release_consistent"] is True,d
assert d["version"]==expected,(d.get("version"),expected)
PY
echo ok

create_partner() {
  email="$1"; suffix="$2"
  payload="$(python3 - "$STAMP" "$email" "$suffix" <<'PY'
import json,sys
stamp,email,suffix=sys.argv[1:]
print(json.dumps({
 "display_name":"START 23.11.4 Workspace "+suffix+" "+stamp,
 "legal_name":"START 23.11.4 Workspace "+suffix+" LLC",
 "brand_name":"Workspace "+suffix,
 "category_id":"cat_006","lifecycle":"PROSPECT",
 "contact_name":"Workspace Owner "+suffix,"contact_email":email,
 "country":"United States","state_region":"New York","city":"New York",
 "onboarding_request_id":"start-23-11-4-"+suffix.lower()+"-"+stamp
}))
PY
)"
  curl -fsS -b "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/partners"
}

printf 'create and activate two isolated Golden Test tenants... '
A="$(create_partner "$EMAIL_A" A)"
B="$(create_partner "$EMAIL_B" B)"
A_ID="$(printf '%s' "$A" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
B_ID="$(printf '%s' "$B" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
test "$A_ID" != "$B_ID"
curl -fsS -b "$ADMIN_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"START-23.11.4 workspace acceptance"}' "$BASE_URL/api/v1/partners/$A_ID" >/dev/null
curl -fsS -b "$ADMIN_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"START-23.11.4 tenant isolation"}' "$BASE_URL/api/v1/partners/$B_ID" >/dev/null
echo ok

printf 'create and authenticate Partner Portal owners... '
owner_payload_a="$(python3 - "$EMAIL_A" "$PASSWORD_A" <<'PY'
import json,sys; print(json.dumps({"name":"Workspace Owner A","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
owner_payload_b="$(python3 - "$EMAIL_B" "$PASSWORD_B" <<'PY'
import json,sys; print(json.dumps({"name":"Workspace Owner B","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
curl -fsS -b "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$owner_payload_a" "$BASE_URL/api/v1/partners/$A_ID/portal-users" >/dev/null
curl -fsS -b "$ADMIN_COOKIE" -H 'Content-Type: application/json' -d "$owner_payload_b" "$BASE_URL/api/v1/partners/$B_ID/portal-users" >/dev/null
login_a="$(python3 - "$EMAIL_A" "$PASSWORD_A" <<'PY'
import json,sys; print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
login_b="$(python3 - "$EMAIL_B" "$PASSWORD_B" <<'PY'
import json,sys; print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$PARTNER_A_COOKIE" -H 'Content-Type: application/json' -d "$login_a" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
curl -fsS -c "$PARTNER_B_COOKIE" -H 'Content-Type: application/json' -d "$login_b" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
echo ok

printf 'design read model exposes presentation-only workspace contract... '
INITIAL="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/design")"
python3 - "$INITIAL" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["presentation_contract"]=="CANONICAL_KEYS_AND_SYSTEM_BEHAVIOR_UNCHANGED",d
assert d["workspace"]["presentation_only"] is True,d
assert d["workspace"]["primary_color"]=="#0B1F3B",d
assert isinstance(d["module_presentations"],list),d
assert len(d["icon_library"])>=10,d
PY
echo ok

printf 'upload tenant-A workspace logo and keep it private until linked... '
python3 - "$PNG" <<'PY'
import base64,sys
raw="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
open(sys.argv[1],"wb").write(base64.b64decode(raw))
PY
MEDIA="$(curl -fsS -b "$PARTNER_A_COOKIE" -F 'alt_text=START 23.11.4 workspace logo' -F "file=@$PNG;type=image/png;filename=workspace-logo.png" "$BASE_URL/partner/api/v1/design/media")"
MEDIA_ID="$(printf '%s' "$MEDIA" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
test "$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/public/v1/cms/media/$MEDIA_ID")" = "404"
echo ok

printf 'save workspace identity, brand colors and ACTIVE default module... '
WORKSPACE_PAYLOAD="$(python3 - "$MEDIA_ID" <<'PY'
import json,sys
print(json.dumps({
 "workspace_name":"Klavierhaus Daily Workspace",
 "logo_media_id":sys.argv[1],
 "primary_color":"#0B1F3B",
 "sidebar_color":"#111827",
 "background_color":"#F8F9FB",
 "accent_color":"#C99A45",
 "text_color":"#1F2937",
 "default_module_key":"workshop_workflow"
}))
PY
)"
WORKSPACE="$(curl -fsS -b "$PARTNER_A_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$WORKSPACE_PAYLOAD" "$BASE_URL/partner/api/v1/design/workspace")"
python3 - "$WORKSPACE" "$MEDIA_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); media=sys.argv[2]
assert d["workspace_name"]=="Klavierhaus Daily Workspace",d
assert d["logo_media_id"]==media,d
assert d["default_module_key"]=="workshop_workflow",d
assert d["presentation_only"] is True,d
PY
curl -fsS "$BASE_URL/public/v1/cms/media/$MEDIA_ID" -o "$MEDIA_OUT"
cmp "$PNG" "$MEDIA_OUT"
echo ok

printf 'contrast protection rejects unreadable workspace branding... '
BAD_CONTRAST="$(python3 - <<'PY'
import json
print(json.dumps({
 "workspace_name":"Unreadable","logo_media_id":"",
 "primary_color":"#111111","sidebar_color":"#FFFFFF",
 "background_color":"#FFFFFF","accent_color":"#CCCCCC","text_color":"#FFFFFF",
 "default_module_key":""
}))
PY
)"
CODE="$(status "$PARTNER_A_COOKIE" PUT "/partner/api/v1/design/workspace" -H 'Content-Type: application/json' -d "$BAD_CONTRAST")"
test "$CODE" = "400"
grep -q 'contrast' "$BODY"
echo ok

printf 'default module cannot point outside the authenticated tenant Marketplace... '
INVALID_DEFAULT="$(python3 - <<'PY'
import json
print(json.dumps({
 "workspace_name":"Klavierhaus Daily Workspace","logo_media_id":"",
 "primary_color":"#0B1F3B","sidebar_color":"#111827",
 "background_color":"#F8F9FB","accent_color":"#C99A45","text_color":"#1F2937",
 "default_module_key":"not_real_module"
}))
PY
)"
CODE="$(status "$PARTNER_A_COOKIE" PUT "/partner/api/v1/design/workspace" -H 'Content-Type: application/json' -d "$INVALID_DEFAULT")"
test "$CODE" = "404"
grep -q 'MODULE_NOT_FOUND' "$BODY"
echo ok

printf 'canonical module metadata remains authoritative before presentation override... '
CANONICAL_BEFORE="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/modules")"
FINANCE_LABEL="$(printf '%s' "$CANONICAL_BEFORE" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next(i for i in d["items"] if i["key"]=="finance"); print(x["label"])')"
test -n "$FINANCE_LABEL"
echo ok

printf 'save tenant-only Finance card presentation... '
MODULE_PAYLOAD='{"display_name":"Business Finance","description":"Company-facing finance workspace. It keeps the canonical HIMATE finance module underneath. Billing and permissions are unchanged.","icon_key":"finance","custom_icon_media_id":"","card_color":"#2E7D32"}'
PRESENTATION="$(curl -fsS -b "$PARTNER_A_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$MODULE_PAYLOAD" "$BASE_URL/partner/api/v1/design/modules/finance")"
python3 - "$PRESENTATION" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["module_key"]=="finance",d
assert d["display_name"]=="Business Finance",d
assert d["icon_key"]=="finance",d
assert d["card_color"]=="#2E7D32",d
assert d["presentation_only"] is True,d
PY
echo ok

printf 'design readback and PostgreSQL persist workspace/module presentation... '
READBACK="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/design")"
python3 - "$READBACK" "$MEDIA_ID" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); media=sys.argv[2]
w=d["workspace"]
assert w["workspace_name"]=="Klavierhaus Daily Workspace" and w["logo_media_id"]==media,w
x=next(i for i in d["module_presentations"] if i["module_key"]=="finance")
assert x["display_name"]=="Business Finance" and x["card_color"]=="#2E7D32",x
PY
DB_WORKSPACE="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' -c "SELECT workspace_name,default_module_key FROM cms.partner_workspace_settings WHERE partner_id='$A_ID';")"
test "$DB_WORKSPACE" = "Klavierhaus Daily Workspace|workshop_workflow"
DB_MODULE="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' -c "SELECT display_name,icon_key,card_color FROM cms.partner_module_presentations WHERE partner_id='$A_ID' AND module_key='finance';")"
test "$DB_MODULE" = "Business Finance|finance|#2E7D32"
echo ok

printf 'presentation override does not mutate canonical Catalog identity or entitlement... '
CANONICAL_AFTER="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/modules")"
python3 - "$CANONICAL_AFTER" "$FINANCE_LABEL" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); label=sys.argv[2]
x=next(i for i in d["items"] if i["key"]=="finance")
assert x["label"]==label,x
assert x["access_state"]=="ACTIVE" and x["executable"] is True,x
assert x["label"]!="Business Finance",x
PY
echo ok

printf 'tenant B cannot reference tenant A workspace media and sees no A presentation... '
CROSS="$(python3 - "$MEDIA_ID" <<'PY'
import json,sys
print(json.dumps({
 "workspace_name":"Tenant B","logo_media_id":sys.argv[1],
 "primary_color":"#0B1F3B","sidebar_color":"#111827",
 "background_color":"#F8F9FB","accent_color":"#C99A45","text_color":"#1F2937",
 "default_module_key":"workshop_workflow"
}))
PY
)"
CODE="$(status "$PARTNER_B_COOKIE" PUT "/partner/api/v1/design/workspace" -H 'Content-Type: application/json' -d "$CROSS")"
test "$CODE" = "403"
grep -q 'MEDIA_SCOPE' "$BODY"
B_DESIGN="$(curl -fsS -b "$PARTNER_B_COOKIE" "$BASE_URL/partner/api/v1/design")"
python3 - "$B_DESIGN" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
assert d["workspace"]["workspace_name"]=="",d
assert all(x.get("module_key")!="finance" for x in d["module_presentations"]),d
PY
echo ok

printf 'Reset to HIMATE default removes only the presentation override... '
curl -fsS -b "$PARTNER_A_COOKIE" -X DELETE "$BASE_URL/partner/api/v1/design/modules/finance" >/dev/null
AFTER_RESET="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/design")"
printf '%s' "$AFTER_RESET" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert all(x.get("module_key")!="finance" for x in d["module_presentations"])'
RESET_COUNT="$(docker compose exec -T postgres psql -U himate -d himate -At -c "SELECT COUNT(*) FROM cms.partner_module_presentations WHERE partner_id='$A_ID' AND module_key='finance';")"
test "$RESET_COUNT" = "0"
echo ok

printf 'workspace and module presentation mutations are audit persisted... '
sleep 1
AUDIT="$(curl -fsS -b "$ADMIN_COOKIE" "$BASE_URL/api/v1/audit/events?partner_id=$A_ID&limit=100")"
python3 - "$AUDIT" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); actions={x["action"] for x in d["items"]}
required={"PARTNER_WORKSPACE_PERSONALIZATION_UPDATED","PARTNER_MODULE_PRESENTATION_UPDATED","PARTNER_MODULE_PRESENTATION_RESET"}
assert required<=actions,(required-actions,actions)
PY
echo ok

echo 'HIMATE START-23.11.4 Partner Workspace & Personalization smoke passed'
