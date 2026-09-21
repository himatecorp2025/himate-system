#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COOKIE_JAR="/tmp/himate-business-completion-cookies.txt"
BODY="/tmp/himate-business-completion-body.json"
rm -f "$COOKIE_JAR" "$BODY"
trap 'rm -f "$COOKIE_JAR" "$BODY"' EXIT

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

printf 'owner login... '
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"Local-Development1!Password","remember":false}' \
  "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'public contact inquiry enters lead store... '
lead_email="business-completion@example.com"
contact="$(curl -fsS -H 'Content-Type: application/json' \
  -d '{"name":"Business Completion Lead","organization":"HIMATE CI","email":"business-completion@example.com","message":"Please contact our organization about a HIMATE partnership and platform rollout.","website":""}' \
  "$BASE_URL/api/v1/public/contact")"
lead_id="$(printf '%s' "$contact" | json_field id)"
test -n "$lead_id"
leads="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/contact/inquiries?q=business-completion%40example.com&limit=25&offset=0")"
printf '%s' "$leads" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["total"]>=1; x=next(i for i in d["items"] if i["email"]=="business-completion@example.com"); assert x["lead_status"]=="NEW"'
echo ok

printf 'contact lead workflow persists... '
updated="$(curl -fsS -b "$COOKIE_JAR" -X PATCH -H 'Content-Type: application/json' \
  -d '{"lead_status":"CONTACTED","assigned_to":"Simon Alex","admin_note":"CI follow-up verified."}' \
  "$BASE_URL/api/v1/contact/inquiries/$lead_id")"
printf '%s' "$updated" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lead_status"]=="CONTACTED"; assert d["assigned_to"]=="Simon Alex"; assert d["admin_note"]=="CI follow-up verified."'
echo ok

printf 'Contact Leads mutation reaches central audit... '
sleep 1
audit="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/audit/events?action=CONTACT_LEAD_UPDATED&limit=50")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert any(x["action"]=="CONTACT_LEAD_UPDATED" and x["resource"]=="contact" for x in d["items"]),d'
echo ok

printf 'SEO Keywords draft and publication... '
seo_payload='{"global_keywords_en":["arts","culture","cultural organizations","HIMATE"],"global_keywords_hu":["művészet","kultúra","kulturális szervezetek","HIMATE"],"organization_name":"HIMATE System","organization_url":"https://www.himate.com","default_og_image_asset_id":""}'
seo_draft="$(curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json' -d "$seo_payload" "$BASE_URL/api/v1/cms/seo/draft")"
printf '%s' "$seo_draft" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "culture" in d["draft"]["global_keywords_en"]; assert "kultúra" in d["draft"]["global_keywords_hu"]'
seo_published="$(curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/seo/publish")"
printf '%s' "$seo_published" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["version"]>=1; assert d["published"]["organization_name"]=="HIMATE System"'
public_seo="$(curl -fsS "$BASE_URL/public/v1/cms/seo?locale=hu_HU")"
printf '%s' "$public_seo" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["locale"]=="hu_HU"; assert "kultúra" in d["global_keywords"]; assert d["organization_url"]=="https://www.himate.com"'
echo ok

printf 'Design Guide draft and publication... '
design_payload='{"logo_media_asset_id":"","navy":"#06172C","gold":"#D7AE62","background":"#F8F9FB","text_color":"#1F2937","heading_font":"Cormorant Garamond","body_font":"Inter","button_radius":8,"navigation":[{"label_en":"Platform","label_hu":"Platform","url":"/platform","visible":true,"sort_order":10},{"label_en":"Modules","label_hu":"Modulok","url":"/modules","visible":true,"sort_order":20},{"label_en":"Contact","label_hu":"Kapcsolat","url":"/contact","visible":true,"sort_order":30}]}'
draft="$(curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json' -d "$design_payload" "$BASE_URL/api/v1/cms/design/draft")"
printf '%s' "$draft" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["draft"]["button_radius"]==8; assert d["draft"]["navigation"][1]["label_hu"]=="Modulok"'
published="$(curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/design/publish")"
printf '%s' "$published" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["version"]>=1; assert d["published"]["gold"]=="#D7AE62"'
public_design="$(curl -fsS "$BASE_URL/public/v1/cms/design")"
printf '%s' "$public_design" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["version"]>=1; assert d["design"]["navigation"][2]["label_hu"]=="Kapcsolat"'
echo ok

printf 'CMS stores independent English and Hungarian variants... '
en_page="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d '{"page_key":"business_locale_ci","name":"Business Locale CI EN","locale":"en_US","version":{"slug":"business-locale-ci","seo":{"title":"HIMATE cultural platform for arts organizations","meta_description":"HIMATE connects arts and cultural organizations with programs, partnerships, evidence, reporting and measurable community impact on one digital platform.","keywords":["arts organizations","cultural platform","community impact"],"canonical":"https://www.himate.com/business-locale-ci","og_title":"HIMATE cultural platform","og_description":"A connected platform for arts and cultural organizations.","og_image_asset_id":"","noindex":true},"sections":[{"id":"hero","component_type":"HERO","heading":"Culture connects people and organizations","body":"Arts organizations use the HIMATE cultural platform to connect programs, partnerships, evidence, reporting, communities and measurable impact. The integrated environment supports cultural teams with structured operations while preserving their identity, context and long-term mission.","media_asset_id":"","cta_label":"","cta_url":"","visible":true,"sort_order":10,"settings":{}}]}}' \
  "$BASE_URL/api/v1/cms/pages")"
en_id="$(printf '%s' "$en_page" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["page"]["locale"]=="en_US"; print(d["page"]["id"])')"
test -n "$en_id"

hu_page="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d '{"page_key":"business_locale_ci","name":"Business Locale CI HU","locale":"hu_HU","version":{"slug":"business-locale-ci","seo":{"title":"HIMATE kulturális platform művészeti szervezeteknek","meta_description":"A HIMATE egy digitális kulturális platform, amely programokat, partnerségeket, bizonyítékokat, jelentéseket és mérhető közösségi hatást kapcsol össze.","keywords":["kulturális platform","művészeti szervezetek","közösségi hatás"],"canonical":"https://www.himate.com/hu/business-locale-ci","og_title":"HIMATE kulturális platform","og_description":"Összekapcsolt platform művészeti és kulturális szervezeteknek.","og_image_asset_id":"","noindex":true},"sections":[{"id":"hero","component_type":"HERO","heading":"A kultúra embereket és szervezeteket kapcsol össze","body":"A művészeti szervezetek a HIMATE kulturális platformon kapcsolhatják össze programjaikat, partnerségeiket, bizonyítékaikat, jelentéseiket, közösségeiket és mérhető hatásukat. Az integrált környezet strukturált működést támogat, miközben megőrzi a kulturális identitást, a kontextust és a hosszú távú küldetést.","media_asset_id":"","cta_label":"Kapcsolat","cta_url":"/contact","visible":true,"sort_order":10,"settings":{}}]}}' \
  "$BASE_URL/api/v1/cms/pages")"
hu_id="$(printf '%s' "$hu_page" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["page"]["locale"]=="hu_HU"; print(d["page"]["id"])')"
test -n "$hu_id"

curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$en_id/preview" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$en_id/publish" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$hu_id/preview" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$hu_id/publish" >/dev/null

printf 'published bilingual CMS variants carry effective SEO... '
en_public="$(curl -fsS "$BASE_URL/public/v1/cms/pages/business-locale-ci?locale=en_US")"
printf '%s' "$en_public" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["locale"]=="en_US"; assert d["seo"]["title"]=="HIMATE cultural platform for arts organizations"; assert "arts organizations" in d["seo"]["keywords"]; assert "culture" in d["seo"]["keywords"]; assert d["seo"]["json_ld"]["inLanguage"]=="en-US"; assert d["alternates"]["en_US"]=="https://www.himate.com/business-locale-ci"; assert d["alternates"]["hu_HU"]=="https://www.himate.com/hu/business-locale-ci"; assert d["sections"][0]["heading"].startswith("Culture connects")'
hu_public="$(curl -fsS "$BASE_URL/public/v1/cms/pages/business-locale-ci?locale=hu_HU")"
printf '%s' "$hu_public" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["locale"]=="hu_HU"; assert d["seo"]["title"].startswith("HIMATE kulturális platform"); assert "kulturális platform" in d["seo"]["keywords"]; assert "kultúra" in d["seo"]["keywords"]; assert d["seo"]["json_ld"]["inLanguage"]=="hu-HU"; assert d["alternates"]["en_US"]=="https://www.himate.com/business-locale-ci"; assert d["alternates"]["hu_HU"]=="https://www.himate.com/hu/business-locale-ci"; assert d["sections"][0]["heading"].startswith("A kultúra")'
echo ok

printf 'automatic SEO audit scores bilingual CMS drafts... '
seo_audit="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/cms/seo/audit")"
printf '%s' "$seo_audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); rows=[x for x in d["items"] if x["page_key"]=="business_locale_ci"]; assert len(rows)==2,rows; assert {x["locale"] for x in rows}=={"en_US","hu_HU"}; assert all(isinstance(x["score"],int) and 0<=x["score"]<=100 for x in rows); assert all(isinstance(x["suggested_keywords"],list) for x in rows); assert d["summary"]["pages"]>=2'
echo ok

printf 'localized manifests isolate language while both variants remain published... '
en_manifest="$(curl -fsS "$BASE_URL/public/v1/cms/manifest?locale=en_US")"
hu_manifest="$(curl -fsS "$BASE_URL/public/v1/cms/manifest?locale=hu_HU")"
printf '%s' "$en_manifest" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next(i for i in d["items"] if i["slug"]=="business-locale-ci"); assert x["locale"]=="en_US"; assert x["canonical"]=="https://www.himate.com/business-locale-ci"'
printf '%s' "$hu_manifest" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next(i for i in d["items"] if i["slug"]=="business-locale-ci"); assert x["locale"]=="hu_HU"; assert x["canonical"]=="https://www.himate.com/hu/business-locale-ci"'
echo ok

printf 'SEO settings mutations reach central audit... '
sleep 1
seo_events="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/audit/events?resource=cms&limit=100")"
printf '%s' "$seo_events" | python3 -c 'import json,sys; d=json.load(sys.stdin); actions={x["action"] for x in d["items"]}; assert "SEO_SETTINGS_DRAFT_SAVED" in actions; assert "SEO_SETTINGS_PUBLISHED" in actions'
echo ok
echo "HIMATE pre-START-22 business completion smoke passed"
