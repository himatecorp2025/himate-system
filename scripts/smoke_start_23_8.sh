#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start238-owner.txt"
PARTNER_A_COOKIE="$TMP_ROOT/himate-start238-partner-a.txt"
PARTNER_B_COOKIE="$TMP_ROOT/himate-start238-partner-b.txt"
BODY="$TMP_ROOT/himate-start238-body.txt"
HEADERS="$TMP_ROOT/himate-start238-headers.txt"
PNG="$TMP_ROOT/himate-start238.png"
MEDIA_OUT="$TMP_ROOT/himate-start238-media.png"
rm -f "$COOKIE" "$PARTNER_A_COOKIE" "$PARTNER_B_COOKIE" "$BODY" "$HEADERS" "$PNG" "$MEDIA_OUT"
trap 'rm -f "$COOKIE" "$PARTNER_A_COOKIE" "$PARTNER_B_COOKIE" "$BODY" "$HEADERS" "$PNG" "$MEDIA_OUT"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"
PAGE_KEY="ci_238_$STAMP"
SLUG="ci-238-$STAMP"
CANONICAL_EN="https://www.himate.com/$SLUG"
CANONICAL_HU="https://www.himate.com/$SLUG?lang=hu"

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

printf 'START-23.8 administrator login... '
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'capture current Design and SEO published state... '
design_before="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/design")"
seo_before="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/seo")"
echo ok

printf 'upload checksum-backed CMS media... '
python3 - "$PNG" <<'PY'
import base64,sys
raw="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
open(sys.argv[1],"wb").write(base64.b64decode(raw))
PY
media="$(curl -fsS -b "$COOKIE" -F 'alt_text=START 23.8 arbitrary section media' -F "file=@$PNG;type=image/png;filename=start238.png" "$BASE_URL/api/v1/cms/media")"
media_id="$(printf '%s' "$media" | json_field id)"
printf '%s' "$media" | python3 -c 'import json,sys,re; d=json.load(sys.stdin); assert d["mime_type"]=="image/png"; assert re.fullmatch(r"[0-9a-f]{64}",d["sha256"])'
test -n "$media_id"
echo ok

printf 'create English arbitrary CMS page draft... '
page_en_payload="$(python3 - "$PAGE_KEY" "$SLUG" "$CANONICAL_EN" "$media_id" "$STAMP" <<'PY'
import json,sys
key,slug,canonical,media,stamp=sys.argv[1:]
print(json.dumps({
 "page_key":key,"name":"START 23.8 English "+stamp,"locale":"en_US",
 "version":{
   "slug":slug,
   "seo":{
     "title":"START 23.8 Dynamic Culture Page",
     "meta_description":"A fully server-rendered arbitrary HIMATE CMS page used for START 23.8 acceptance.",
     "keywords":["culture","dynamic cms","start238"],
     "canonical":canonical,
     "og_title":"START 23.8 Dynamic Culture",
     "og_description":"CMS arbitrary page preview and publish proof.",
     "og_image_asset_id":media,
     "noindex":False
   },
   "sections":[
     {"id":"brand-story","component_type":"FEATURE","heading":"ARBITRARY SECTION 238","body":"This section did not exist in any source HTML template.","media_asset_id":media,"cta_label":"Explore culture","cta_url":"/impact","visible":True,"sort_order":10,"settings":{"layout":"split"}},
     {"id":"hidden-draft","component_type":"TEXT","heading":"HIDDEN 238","body":"This must not render.","media_asset_id":"","cta_label":"","cta_url":"","visible":False,"sort_order":20,"settings":{}}
   ]
 }
},separators=(",",":")))
PY
)"
created_en="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$page_en_payload" "$BASE_URL/api/v1/cms/pages")"
page_en_id="$(printf '%s' "$created_en" | python3 -c 'import json,sys; print(json.load(sys.stdin)["page"]["id"])')"
test -n "$page_en_id"
echo ok

printf 'full HTML preview renders arbitrary section and private media... '
preview_en="$(curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/pages/$page_en_id/preview")"
preview_token="$(printf '%s' "$preview_en" | json_field preview_token)"
preview_html_path="$(printf '%s' "$preview_en" | json_field preview_html_path)"
test -n "$preview_token"
printf '%s' "$preview_html_path" | grep -q '^/cms-preview/'
curl -fsS -D "$HEADERS" "$BASE_URL$preview_html_path" -o "$BODY"
grep -qi '^X-Himate-Preview: cms' "$HEADERS"
grep -qi '^X-Himate-SSR: preview' "$HEADERS"
grep -qi '^X-Robots-Tag: noindex, nofollow' "$HEADERS"
grep -q 'HIMATE SSR:PREVIEW' "$BODY"
grep -q 'data-cms-section="brand-story"' "$BODY"
grep -q 'himate-cms-dynamic' "$BODY"
grep -q 'ARBITRARY SECTION 238' "$BODY"
grep -q 'This section did not exist in any source HTML template.' "$BODY"
grep -q "/preview/v1/cms/media/$media_id" "$BODY"
if grep -q 'HIDDEN 238' "$BODY"; then exit 1; fi
curl -fsS "$BASE_URL/preview/v1/cms/media/$media_id?slug=$SLUG&token=$preview_token" -o "$MEDIA_OUT"
cmp "$PNG" "$MEDIA_OUT"
echo ok

printf 'unpublished arbitrary slug is not publicly rendered... '
unpublished_code="$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/$SLUG")"
test "$unpublished_code" = "302"
echo ok

printf 'publish English page and prove initial SSR plus public media... '
published_en="$(curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/pages/$page_en_id/publish")"
published_en_v1="$(printf '%s' "$published_en" | json_field id)"
test -n "$published_en_v1"
curl -fsS -D "$HEADERS" "$BASE_URL/$SLUG" -o "$BODY"
grep -qi '^X-Himate-SSR: published' "$HEADERS"
grep -q 'HIMATE SSR:PUBLISHED' "$BODY"
grep -q '<title>START 23.8 Dynamic Culture Page</title>' "$BODY"
grep -q 'ARBITRARY SECTION 238' "$BODY"
grep -q 'data-cms-section="brand-story"' "$BODY"
grep -q "/public/v1/cms/media/$media_id" "$BODY"
if grep -q 'HIDDEN 238' "$BODY"; then exit 1; fi
curl -fsS "$BASE_URL/public/v1/cms/media/$media_id" -o "$MEDIA_OUT"
cmp "$PNG" "$MEDIA_OUT"
echo ok

printf 'create, preview and publish Hungarian sibling on same slug... '
page_hu_payload="$(python3 - "$PAGE_KEY" "$SLUG" "$CANONICAL_HU" "$media_id" "$STAMP" <<'PY'
import json,sys
key,slug,canonical,media,stamp=sys.argv[1:]
print(json.dumps({
 "page_key":key,"name":"START 23.8 Magyar "+stamp,"locale":"hu_HU",
 "version":{
   "slug":slug,
   "seo":{
     "title":"START 23.8 Dinamikus Kultúra Oldal",
     "meta_description":"Teljes szerveroldali magyar CMS oldal a START 23.8 elfogadási teszthez.",
     "keywords":["kultúra","dinamikus cms","start238"],
     "canonical":canonical,
     "og_title":"START 23.8 Dinamikus Kultúra",
     "og_description":"Magyar CMS publikálási bizonyíték.",
     "og_image_asset_id":media,
     "noindex":False
   },
   "sections":[
     {"id":"brand-story","component_type":"FEATURE","heading":"TETSZŐLEGES SZEKCIÓ 238","body":"Ez a szekció nem létezett a forrás HTML sablonban.","media_asset_id":media,"cta_label":"Hatás megtekintése","cta_url":"/impact","visible":True,"sort_order":10,"settings":{}}
   ]
 }
},separators=(",",":")))
PY
)"
created_hu="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$page_hu_payload" "$BASE_URL/api/v1/cms/pages")"
page_hu_id="$(printf '%s' "$created_hu" | python3 -c 'import json,sys; print(json.load(sys.stdin)["page"]["id"])')"
curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/pages/$page_hu_id/preview" >/dev/null
curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/pages/$page_hu_id/publish" >/dev/null
curl -fsS -D "$HEADERS" "$BASE_URL/$SLUG?lang=hu" -o "$BODY"
grep -qi '^Content-Language: hu' "$HEADERS"
grep -q '<html lang="hu">' "$BODY"
grep -q 'TETSZŐLEGES SZEKCIÓ 238' "$BODY"
echo ok

printf 'draft save, second publish and rollback restore public HTML... '
second_payload="$(python3 - "$SLUG" "$CANONICAL_EN" "$media_id" <<'PY'
import json,sys
slug,canonical,media=sys.argv[1:]
print(json.dumps({
 "slug":slug,
 "seo":{"title":"START 23.8 Dynamic Culture Page","meta_description":"A fully server-rendered arbitrary HIMATE CMS page used for START 23.8 acceptance.","keywords":["culture","dynamic cms","start238"],"canonical":canonical,"og_title":"START 23.8 Dynamic Culture","og_description":"CMS arbitrary page preview and publish proof.","og_image_asset_id":media,"noindex":False},
 "sections":[{"id":"brand-story","component_type":"FEATURE","heading":"SECOND PUBLISHED SECTION 238","body":"Second published CMS body.","media_asset_id":media,"cta_label":"Explore culture","cta_url":"/impact","visible":True,"sort_order":10,"settings":{}}]
},separators=(",",":")))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$second_payload" "$BASE_URL/api/v1/cms/pages/$page_en_id/draft" >/dev/null
curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/pages/$page_en_id/preview" >/dev/null
curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/pages/$page_en_id/publish" >/dev/null
curl -fsS "$BASE_URL/$SLUG" -o "$BODY"
grep -q 'SECOND PUBLISHED SECTION 238' "$BODY"
rollback_payload="$(python3 - "$published_en_v1" <<'PY'
import json,sys
print(json.dumps({"version_id":sys.argv[1]}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$rollback_payload" "$BASE_URL/api/v1/cms/pages/$page_en_id/rollback" >/dev/null
curl -fsS "$BASE_URL/$SLUG" -o "$BODY"
grep -q 'ARBITRARY SECTION 238' "$BODY"
if grep -q 'SECOND PUBLISHED SECTION 238' "$BODY"; then exit 1; fi
echo ok

printf 'save Design Guide draft and verify authoritative readback... '
design_payload="$(python3 - "$SLUG" "$media_id" <<'PY'
import json,sys
slug,media=sys.argv[1:]
print(json.dumps({
 "logo_media_asset_id":media if False else "",
 "assets":{
   "header_wordmark":media,
   "footer_wordmark":media,
   "favicon":media,
   "app_icon":media,
   "login_logo":media,
   "email_logo":media
 },
 "layout_key":"modern_grid",
 "navy":"#123456",
 "gold":"#C59A42",
 "background":"#F2EFE8",
 "text_color":"#243447",
 "heading_font":"Georgia",
 "body_font":"Arial",
 "button_radius":17,
 "navigation":[
   {"label_en":"Culture Lab 238","label_hu":"Kultúralabor 238","url":"/"+slug,"visible":True,"sort_order":10},
   {"label_en":"Impact","label_hu":"Hatás","url":"/impact","visible":True,"sort_order":20},
   {"label_en":"Contact","label_hu":"Kapcsolat","url":"/contact","visible":True,"sort_order":30}
 ]
},separators=(",",":")))
PY
)"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$design_payload" "$BASE_URL/api/v1/cms/design/draft" >/dev/null
design_read="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/design")"
printf '%s' "$design_read" | python3 -c 'import json,sys; d=json.load(sys.stdin)["draft"]; assert d["navy"]=="#123456"; assert d["button_radius"]==17; assert d["layout_key"]=="modern_grid"; assert d["assets"]["favicon"]==sys.argv[1]; assert d["assets"]["login_logo"]==sys.argv[1]; assert d["assets"]["email_logo"]==sys.argv[1]; assert d["navigation"][0]["label_en"]=="Culture Lab 238"' "$media_id"
echo ok

printf 'desktop/tablet/mobile Design preview renders the real website... '
design_preview="$(curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/design/preview")"
desktop_path="$(printf '%s' "$design_preview" | json_field desktop_path)"
tablet_path="$(printf '%s' "$design_preview" | json_field tablet_path)"
mobile_path="$(printf '%s' "$design_preview" | json_field mobile_path)"
for entry in "$desktop_path|1440px" "$tablet_path|834px" "$mobile_path|390px"; do
  path="${entry%%|*}"
  width="${entry##*|}"
  curl -fsS -D "$HEADERS" "$BASE_URL$path" -o "$BODY"
  grep -qi '^X-Himate-Preview: design' "$HEADERS"
  grep -qi '^X-Robots-Tag: noindex, nofollow' "$HEADERS"
  grep -q "width:$width" "$BODY"
  grep -q 'HIMATE Design Preview' "$BODY"
done
raw_design_path="$desktop_path&raw=1"
curl -fsS "$BASE_URL$raw_design_path" -o "$BODY"
grep -q 'data-himate-design' "$BODY"
grep -q -- '--navy:#123456' "$BODY"
grep -q 'Culture Lab 238' "$BODY"
grep -q 'border-radius:17px' "$BODY"
grep -q 'rel="icon"' "$BODY"
grep -q 'apple-touch-icon' "$BODY"
grep -q "/preview/v1/cms/design/media/$media_id?token=" "$BODY"
if grep -q "/public/v1/cms/media/$media_id" "$BODY"; then
  echo "Design preview leaked draft brand asset through public media path" >&2
  exit 1
fi
echo ok

printf 'publish Design Guide and prove public initial HTML mutation... '
curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/design/publish" >/dev/null
curl -fsS -D "$HEADERS" "$BASE_URL/$SLUG" -o "$BODY"
grep -qi '^X-Himate-Design: published' "$HEADERS"
grep -q -- '--navy:#123456' "$BODY"
grep -q 'Culture Lab 238' "$BODY"
grep -q 'border-radius:17px' "$BODY"
curl -fsS "$BASE_URL/$SLUG?lang=hu" -o "$BODY"
grep -q 'Kultúralabor 238' "$BODY"
echo ok

printf 'save and publish bilingual global SEO settings... '
seo_payload='{"global_keywords_en":["start238-global","cultural technology","arts platform"],"global_keywords_hu":["start238-global-hu","kulturális technológia","művészeti platform"],"organization_name":"START 23.8 Cultural Network","organization_url":"https://www.himate.com","default_og_image_asset_id":""}'
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$seo_payload" "$BASE_URL/api/v1/cms/seo/draft" >/dev/null
seo_read="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/seo")"
printf '%s' "$seo_read" | python3 -c 'import json,sys; d=json.load(sys.stdin)["draft"]; assert "start238-global" in d["global_keywords_en"]; assert d["organization_name"]=="START 23.8 Cultural Network"'
curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/seo/publish" >/dev/null
echo ok

printf 'SEO publish is present in public initial HTML, JSON-LD and fallback HTML... '
curl -fsS "$BASE_URL/$SLUG" -o "$BODY"
grep -q 'start238-global' "$BODY"
grep -q 'START 23.8 Cultural Network' "$BODY"
grep -q 'application/ld+json' "$BODY"
curl -fsS "$BASE_URL/contact" -o "$BODY"
grep -q 'start238-global' "$BODY"
grep -q 'data-himate-seo="organization"' "$BODY"
grep -q 'START 23.8 Cultural Network' "$BODY"
echo ok

printf 'page version history and audit preserve create/draft/preview/publish/rollback lineage... '
versions="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/pages/$page_en_id/versions")"
printf '%s' "$versions" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert len(d["items"])>=6; assert any(x["rollback_of_version_id"] for x in d["items"])'
audit="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/pages/$page_en_id/audit")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); actions={x["action"] for x in d["items"]}; required={"PAGE_CREATED","DRAFT_SAVED","PREVIEW_CREATED","PUBLISHED","ROLLBACK_PUBLISHED"}; assert required<=actions,(required-actions,actions)'
echo ok

printf 'create two isolated partner design tenants... '
PARTNER_A_PASSWORD="$(python3 -c 'import secrets; print("Pta8!"+secrets.token_urlsafe(24))')"
PARTNER_B_PASSWORD="$(python3 -c 'import secrets; print("Ptb8!"+secrets.token_urlsafe(24))')"
partner_a_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
stamp=sys.argv[1]
print(json.dumps({
  "display_name":"START 23.8 Design Tenant A "+stamp,
  "legal_name":"START 23.8 Design Tenant A LLC",
  "brand_name":"Theme Tenant A",
  "contact_name":"Theme Owner A",
  "contact_email":"theme-owner-a-"+stamp+"@example.com",
  "country":"US"
}))
PY
)"
partner_b_payload="$(python3 - "$STAMP" <<'PY'
import json,sys
stamp=sys.argv[1]
print(json.dumps({
  "display_name":"START 23.8 Design Tenant B "+stamp,
  "legal_name":"START 23.8 Design Tenant B LLC",
  "brand_name":"Theme Tenant B",
  "contact_name":"Theme Owner B",
  "contact_email":"theme-owner-b-"+stamp+"@example.com",
  "country":"US"
}))
PY
)"
partner_a="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$partner_a_payload" "$BASE_URL/api/v1/partners")"
partner_b="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$partner_b_payload" "$BASE_URL/api/v1/partners")"
partner_a_id="$(printf '%s' "$partner_a" | json_field id)"
partner_b_id="$(printf '%s' "$partner_b" | json_field id)"
test -n "$partner_a_id"
test -n "$partner_b_id"
test "$partner_a_id" != "$partner_b_id"
echo ok

printf 'bootstrap and authenticate tenant-scoped partner owners... '
partner_a_email="ci-start238-owner-a-$STAMP@example.com"
partner_b_email="ci-start238-owner-b-$STAMP@example.com"
owner_a_payload="$(python3 - "$partner_a_email" "$PARTNER_A_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 23.8 Theme Owner A","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
owner_b_payload="$(python3 - "$partner_b_email" "$PARTNER_B_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 23.8 Theme Owner B","email":sys.argv[1],"password":sys.argv[2],"role":"owner"}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$owner_a_payload" "$BASE_URL/api/v1/partners/$partner_a_id/portal-users" >/dev/null
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$owner_b_payload" "$BASE_URL/api/v1/partners/$partner_b_id/portal-users" >/dev/null
login_a="$(python3 - "$partner_a_email" "$PARTNER_A_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
login_b="$(python3 - "$partner_b_email" "$PARTNER_B_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$PARTNER_A_COOKIE" -H 'Content-Type: application/json' -d "$login_a" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
curl -fsS -c "$PARTNER_B_COOKIE" -H 'Content-Type: application/json' -d "$login_b" "$BASE_URL/partner/api/v1/auth/login" >/dev/null
me_a="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/auth/me")"
printf '%s' "$me_a" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1]; assert "*" in d["permissions"]' "$partner_a_id"
echo ok

printf 'partner design catalog starts with selectable visual-only themes... '
design_a_initial="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/design")"
printf '%s' "$design_a_initial" | python3 -c 'import json,sys; d=json.load(sys.stdin); ids={x["id"] for x in d["profiles"]}; assert {"theme_classic_editorial","theme_modern_grid","theme_minimal"}<=ids; assert d["active_profile_id"]=="theme_classic_editorial"; assert d["content_binding"]=="UNCHANGED"; assert d["mechanics_binding"]=="UNCHANGED"'
echo ok

printf 'capture partner content and mechanics fingerprint before any theme switch... '
company_before="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/company")"
modules_before="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/modules")"
company_fp_before="$(printf '%s' "$company_before" | python3 -c 'import json,sys,hashlib; d=json.load(sys.stdin); keep={k:d.get(k) for k in ("id","display_name","legal_name","brand_name","lifecycle","country","city","primary_domain")}; print(hashlib.sha256(json.dumps(keep,sort_keys=True,separators=(",",":")).encode()).hexdigest())')"
modules_fp_before="$(printf '%s' "$modules_before" | python3 -c 'import json,sys,hashlib; d=json.load(sys.stdin); rows=sorted((str(x.get("key","")),str(x.get("status","")),bool(x.get("visible_to_partner",True)),bool(x.get("included_in_base",False))) for x in d.get("items",[])); print(hashlib.sha256(json.dumps(rows,separators=(",",":")).encode()).hexdigest())')"
test -n "$company_fp_before"
test -n "$modules_fp_before"
echo ok

printf 'tenant A uploads checksum-backed partner-owned brand asset... '
partner_media="$(curl -fsS -b "$PARTNER_A_COOKIE" -F 'alt_text=START 23.8 partner theme asset' -F "file=@$PNG;type=image/png;filename=partner-theme.png" "$BASE_URL/partner/api/v1/design/media")"
partner_media_id="$(printf '%s' "$partner_media" | json_field id)"
test -n "$partner_media_id"
printf '%s' "$partner_media" | python3 -c 'import json,sys,re; d=json.load(sys.stdin); assert d["mime_type"]=="image/png"; assert re.fullmatch(r"[0-9a-f]{64}",d["sha256"])'
private_partner_media="$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/public/v1/cms/media/$partner_media_id")"
test "$private_partner_media" = "404"
echo ok

printf 'tenant A creates and edits a custom visual-only theme profile... '
custom_theme_payload="$(python3 - "$partner_media_id" <<'PY'
import json,sys
media=sys.argv[1]
print(json.dumps({
  "name":"START 23.8 Custom Grid",
  "description":"Tenant A custom theme; content and mechanics are external bindings.",
  "theme":{
    "layout_key":"modern_grid",
    "navy":"#17365D",
    "gold":"#C99A45",
    "background":"#F4F6F8",
    "text_color":"#182230",
    "heading_font":"Inter",
    "body_font":"Inter",
    "button_radius":14,
    "assets":{
      "header_wordmark":media,
      "footer_wordmark":media,
      "favicon":media,
      "app_icon":media,
      "login_logo":media,
      "email_logo":media
    }
  }
},separators=(",",":")))
PY
)"
custom_profile="$(curl -fsS -b "$PARTNER_A_COOKIE" -H 'Content-Type: application/json' -d "$custom_theme_payload" "$BASE_URL/partner/api/v1/design/profiles")"
custom_profile_id="$(printf '%s' "$custom_profile" | json_field id)"
test -n "$custom_profile_id"
updated_theme_payload="$(python3 - "$partner_media_id" <<'PY'
import json,sys
media=sys.argv[1]
print(json.dumps({
  "name":"START 23.8 Custom Grid Updated",
  "description":"Updated tenant theme with the same external content/mechanics bindings.",
  "theme":{
    "layout_key":"modern_grid",
    "navy":"#17365D",
    "gold":"#D2A84A",
    "background":"#F4F6F8",
    "text_color":"#182230",
    "heading_font":"Inter",
    "body_font":"Inter",
    "button_radius":16,
    "assets":{
      "header_wordmark":media,
      "footer_wordmark":media,
      "favicon":media,
      "app_icon":media,
      "login_logo":media,
      "email_logo":media
    }
  }
},separators=(",",":")))
PY
)"
updated_profile="$(curl -fsS -b "$PARTNER_A_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$updated_theme_payload" "$BASE_URL/partner/api/v1/design/profiles/$custom_profile_id")"
printf '%s' "$updated_profile" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["name"]=="START 23.8 Custom Grid Updated"; assert d["theme"]["gold"]=="#D2A84A"; assert d["theme"]["button_radius"]==16'
echo ok

printf 'tenant B cannot reference tenant A brand asset in its own theme... '
cross_tenant_payload="$(python3 - "$partner_media_id" <<'PY'
import json,sys
print(json.dumps({
  "name":"Illegal Cross Tenant Theme",
  "description":"Must fail",
  "theme":{
    "layout_key":"minimal",
    "navy":"#111827",
    "gold":"#B8893C",
    "background":"#FFFFFF",
    "text_color":"#111827",
    "heading_font":"Georgia",
    "body_font":"Arial",
    "button_radius":2,
    "assets":{"header_wordmark":sys.argv[1]}
  }
}))
PY
)"
cross_tenant_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$PARTNER_B_COOKIE" -H 'Content-Type: application/json' -d "$cross_tenant_payload" "$BASE_URL/partner/api/v1/design/profiles")"
test "$cross_tenant_code" = "400"
grep -q 'does not belong to this partner' "$BODY"
echo ok

printf 'catalog theme activation changes only the active design pointer... '
catalog_activation="$(curl -fsS -b "$PARTNER_A_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/partner/api/v1/design/profiles/theme_minimal/activate")"
printf '%s' "$catalog_activation" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["active_profile_id"]=="theme_minimal"; assert d["content_binding"]=="UNCHANGED"; assert d["mechanics_binding"]=="UNCHANGED"'
public_minimal="$(curl -fsS "$BASE_URL/public/v1/cms/partner-design/$partner_a_id")"
printf '%s' "$public_minimal" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["profile_id"]=="theme_minimal"; assert d["theme"]["layout_key"]=="minimal"'
echo ok

printf 'custom theme activation preserves content and mechanical state... '
custom_activation="$(curl -fsS -b "$PARTNER_A_COOKIE" -H 'Content-Type: application/json' -d '{}' "$BASE_URL/partner/api/v1/design/profiles/$custom_profile_id/activate")"
printf '%s' "$custom_activation" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["active_profile_id"]==sys.argv[1]; assert d["content_binding"]=="UNCHANGED"; assert d["mechanics_binding"]=="UNCHANGED"' "$custom_profile_id"
company_after="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/company")"
modules_after="$(curl -fsS -b "$PARTNER_A_COOKIE" "$BASE_URL/partner/api/v1/modules")"
company_fp_after="$(printf '%s' "$company_after" | python3 -c 'import json,sys,hashlib; d=json.load(sys.stdin); keep={k:d.get(k) for k in ("id","display_name","legal_name","brand_name","lifecycle","country","city","primary_domain")}; print(hashlib.sha256(json.dumps(keep,sort_keys=True,separators=(",",":")).encode()).hexdigest())')"
modules_fp_after="$(printf '%s' "$modules_after" | python3 -c 'import json,sys,hashlib; d=json.load(sys.stdin); rows=sorted((str(x.get("key","")),str(x.get("status","")),bool(x.get("visible_to_partner",True)),bool(x.get("included_in_base",False))) for x in d.get("items",[])); print(hashlib.sha256(json.dumps(rows,separators=(",",":")).encode()).hexdigest())')"
test "$company_fp_after" = "$company_fp_before"
test "$modules_fp_after" = "$modules_fp_before"
public_custom="$(curl -fsS "$BASE_URL/public/v1/cms/partner-design/$partner_a_id")"
printf '%s' "$public_custom" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["profile_id"]==sys.argv[1]; assert d["theme"]["layout_key"]=="modern_grid"; assert d["theme"]["assets"]["favicon"]==sys.argv[2]' "$custom_profile_id" "$partner_media_id"
curl -fsS "$BASE_URL/public/v1/cms/media/$partner_media_id" -o "$MEDIA_OUT"
cmp "$PNG" "$MEDIA_OUT"
echo ok

printf 'partner theme mutations are centrally audited with tenant scope... '
sleep 1
partner_design_audit="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/audit/events?partner_id=$partner_a_id&limit=100")"
printf '%s' "$partner_design_audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); actions={x["action"] for x in d["items"]}; required={"PARTNER_DESIGN_MEDIA_UPLOADED","PARTNER_THEME_PROFILE_CREATED","PARTNER_THEME_PROFILE_UPDATED","PARTNER_THEME_ACTIVATED"}; assert required<=actions,(required-actions,actions); assert all(x["partner_id"]==sys.argv[1] for x in d["items"])' "$partner_a_id"
echo ok

printf 'restore prior published Design and SEO values for later acceptance phases... '
design_restore="$(printf '%s' "$design_before" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)["published"],separators=(",",":")))' )"
seo_restore="$(printf '%s' "$seo_before" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)["published"],separators=(",",":")))' )"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$design_restore" "$BASE_URL/api/v1/cms/design/draft" >/dev/null
curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/design/publish" >/dev/null
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$seo_restore" "$BASE_URL/api/v1/cms/seo/draft" >/dev/null
curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/seo/publish" >/dev/null
echo ok

echo "HIMATE START-23.8 CMS, Design & SEO Completion smoke passed"
