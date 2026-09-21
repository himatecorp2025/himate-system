#!/usr/bin/env sh
set -eu

if [ "$#" -gt 0 ]; then BASE_URL="$1"; else BASE_URL="http://127.0.0.1:8080"; fi
COOKIE_JAR="/tmp/himate-start-16-cookies.txt"
BODY="/tmp/himate-start-16-body.json"
PNG="/tmp/himate-start-16.png"
DOWNLOADED="/tmp/himate-start-16-downloaded.png"
rm -f "$COOKIE_JAR" "$BODY" "$PNG" "$DOWNLOADED"
trap 'rm -f "$COOKIE_JAR" "$BODY" "$PNG" "$DOWNLOADED"' EXIT

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

printf 'login... '
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"Local-Development1!Password"}' \
  "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'CMS service health... '
health="$(curl -fsS "$BASE_URL/api/v1/health")"
printf '%s' "$health" | grep -q '"cms":"ok"'
echo ok

printf 'unpublished page is absent from public API... '
code="$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/public/v1/cms/pages/ci-cms")"
test "$code" = "404"
echo ok

printf 'reject disguised unsafe media... '
printf '<html>not an image</html>' > "$PNG"
bad_media_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" \
  -F 'alt_text=bad' -F "file=@$PNG;type=image/png;filename=bad.png" \
  "$BASE_URL/api/v1/cms/media")"
test "$bad_media_code" = "415"
echo ok

printf 'upload checksum-backed CMS image... '
python3 - "$PNG" <<'PY'
import base64,sys
raw="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
open(sys.argv[1],"wb").write(base64.b64decode(raw))
PY
media="$(curl -fsS -b "$COOKIE_JAR" \
  -F 'alt_text=CMS smoke pixel' -F "file=@$PNG;type=image/png;filename=pixel.png" \
  "$BASE_URL/api/v1/cms/media")"
media_id="$(printf '%s' "$media" | json_field id)"
test -n "$media_id"
printf '%s' "$media" | python3 -c 'import json,sys,re; d=json.load(sys.stdin); assert d["mime_type"]=="image/png"; assert re.fullmatch(r"[0-9a-f]{64}",d["sha256"])'
private_media="$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/public/v1/cms/media/$media_id")"
test "$private_media" = "404"
echo ok

printf 'create versioned CMS draft... '
page="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d "{\"page_key\":\"ci_cms\",\"name\":\"CMS CI Page\",\"version\":{\"slug\":\"ci-cms\",\"seo\":{\"title\":\"CMS CI Page\",\"meta_description\":\"CMS CI metadata for deterministic acceptance testing.\",\"canonical\":\"https://www.himate.com/ci-cms\",\"og_title\":\"CMS CI\",\"og_description\":\"CMS preview\",\"og_image_asset_id\":\"$media_id\",\"noindex\":false},\"sections\":[{\"id\":\"hero\",\"component_type\":\"HERO\",\"heading\":\"First Published Heading\",\"body\":\"Visible body\",\"media_asset_id\":\"$media_id\",\"cta_label\":\"Contact\",\"cta_url\":\"/contact\",\"visible\":true,\"sort_order\":10,\"settings\":{}},{\"id\":\"hidden-note\",\"component_type\":\"TEXT\",\"heading\":\"Hidden Draft Content\",\"body\":\"This content must remain stored but never enter public output.\",\"media_asset_id\":\"\",\"cta_label\":\"\",\"cta_url\":\"\",\"visible\":false,\"sort_order\":20,\"settings\":{}}]}}" \
  "$BASE_URL/api/v1/cms/pages")"
page_id="$(printf '%s' "$page" | python3 -c 'import json,sys; print(json.load(sys.stdin)["page"]["id"])')"
test -n "$page_id"
echo ok

printf 'preview token protects preview content... '
preview="$(curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$page_id/preview")"
token="$(printf '%s' "$preview" | json_field preview_token)"
test -n "$token"
bad_preview="$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/preview/v1/cms/pages/ci-cms?token=wrong")"
test "$bad_preview" = "404"
preview_body="$(curl -fsS "$BASE_URL/preview/v1/cms/pages/ci-cms?token=$token")"
printf '%s' "$preview_body" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["content_model_version"]==1; assert d["state"]=="PREVIEW"; assert len(d["sections"])==1; assert d["sections"][0]["heading"]=="First Published Heading"; forbidden={"created_by","published_by","page_id","source_version_id","rollback_of_version_id","id"}; assert not forbidden.intersection(d), (forbidden.intersection(d),d)'
curl -fsS "$BASE_URL/preview/v1/cms/media/$media_id?slug=ci-cms&token=$token" -o "$DOWNLOADED"
cmp "$PNG" "$DOWNLOADED"
bad_preview_media="$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/preview/v1/cms/media/$media_id?slug=ci-cms&token=wrong")"
test "$bad_preview_media" = "404"
still_private="$(curl -sS -o "$BODY" -w '%{http_code}' "$BASE_URL/public/v1/cms/pages/ci-cms")"
test "$still_private" = "404"
echo ok

printf 'stale preview cannot publish newer draft... '
curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json' \
  -d "{\"slug\":\"ci-cms\",\"seo\":{\"title\":\"CMS CI Page\",\"meta_description\":\"CMS CI metadata for deterministic acceptance testing.\",\"canonical\":\"https://www.himate.com/ci-cms\",\"og_title\":\"CMS CI\",\"og_description\":\"CMS preview\",\"og_image_asset_id\":\"$media_id\",\"noindex\":false},\"sections\":[{\"id\":\"hero\",\"component_type\":\"HERO\",\"heading\":\"First Published Heading\",\"body\":\"Visible body updated after preview\",\"media_asset_id\":\"$media_id\",\"cta_label\":\"Contact\",\"cta_url\":\"/contact\",\"visible\":true,\"sort_order\":10,\"settings\":{}},{\"id\":\"hidden-note\",\"component_type\":\"TEXT\",\"heading\":\"Hidden Draft Content\",\"body\":\"Still preserved\",\"media_asset_id\":\"\",\"cta_label\":\"\",\"cta_url\":\"\",\"visible\":false,\"sort_order\":20,\"settings\":{}}]}" \
  "$BASE_URL/api/v1/cms/pages/$page_id/draft" >/dev/null
stale_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$page_id/publish")"
test "$stale_code" = "409"
echo ok

printf 'fresh preview publishes and hidden content stays private... '
preview2="$(curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$page_id/preview")"
token2="$(printf '%s' "$preview2" | json_field preview_token)"
test -n "$token2"
published1="$(curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$page_id/publish")"
published1_id="$(printf '%s' "$published1" | json_field id)"
test -n "$published1_id"
public1="$(curl -fsS "$BASE_URL/public/v1/cms/pages/ci-cms")"
printf '%s' "$public1" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["state"]=="PUBLISHED"; assert d["content_model_version"]==1; assert len(d["sections"])==1; assert d["sections"][0]["heading"]=="First Published Heading"; assert d["seo"]["title"]=="CMS CI Page"; forbidden={"created_by","published_by","page_id","source_version_id","rollback_of_version_id","id"}; assert not forbidden.intersection(d), (forbidden.intersection(d),d)'
if printf '%s' "$public1" | grep -q 'Hidden Draft Content'; then exit 1; fi
detail="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/cms/pages/$page_id")"
printf '%s' "$detail" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert len(d["published"]["sections"])==2; assert any(x["visible"] is False for x in d["published"]["sections"])'
curl -fsS "$BASE_URL/public/v1/cms/media/$media_id" -o "$DOWNLOADED"
cmp "$PNG" "$DOWNLOADED"
echo ok

printf 'draft changes never leak into public API... '
curl -fsS -b "$COOKIE_JAR" -X PUT -H 'Content-Type: application/json' \
  -d "{\"slug\":\"ci-cms\",\"seo\":{\"title\":\"CMS CI Page Draft Secret\",\"meta_description\":\"Draft-only metadata that must not leak before publishing.\",\"canonical\":\"https://www.himate.com/ci-cms\",\"og_title\":\"Draft Secret\",\"og_description\":\"Draft\",\"og_image_asset_id\":\"$media_id\",\"noindex\":false},\"sections\":[{\"id\":\"hero\",\"component_type\":\"HERO\",\"heading\":\"DRAFT SECRET LEAK TEST\",\"body\":\"Not public\",\"media_asset_id\":\"$media_id\",\"cta_label\":\"Contact\",\"cta_url\":\"/contact\",\"visible\":true,\"sort_order\":10,\"settings\":{}}]}" \
  "$BASE_URL/api/v1/cms/pages/$page_id/draft" >/dev/null
public_after_draft="$(curl -fsS "$BASE_URL/public/v1/cms/pages/ci-cms")"
if printf '%s' "$public_after_draft" | grep -q 'DRAFT SECRET LEAK TEST'; then exit 1; fi
echo ok

printf 'publish validation blocks incomplete content... '
bad_page="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d '{"page_key":"ci_incomplete","name":"Incomplete CMS Page","version":{"slug":"ci-incomplete","seo":{"title":"","meta_description":"","canonical":"","og_title":"","og_description":"","og_image_asset_id":"","noindex":true},"sections":[]}}' \
  "$BASE_URL/api/v1/cms/pages")"
bad_page_id="$(printf '%s' "$bad_page" | python3 -c 'import json,sys; print(json.load(sys.stdin)["page"]["id"])')"
curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$bad_page_id/preview" >/dev/null
bad_publish="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$bad_page_id/publish")"
test "$bad_publish" = "400"
echo ok

printf 'duplicate canonical and slug are blocked on publish... '
conflict_page="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d '{"page_key":"ci_conflict","name":"Conflict CMS Page","version":{"slug":"ci-cms","seo":{"title":"Conflicting CMS Page","meta_description":"This page intentionally conflicts with the published canonical.","canonical":"https://www.himate.com/ci-cms","og_title":"","og_description":"","og_image_asset_id":"","noindex":false},"sections":[{"id":"hero","component_type":"HERO","heading":"Conflict","body":"Conflict","media_asset_id":"","cta_label":"","cta_url":"","visible":true,"sort_order":10,"settings":{}}]}}' \
  "$BASE_URL/api/v1/cms/pages")"
conflict_id="$(printf '%s' "$conflict_page" | python3 -c 'import json,sys; print(json.load(sys.stdin)["page"]["id"])')"
curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$conflict_id/preview" >/dev/null
conflict_code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$conflict_id/publish")"
test "$conflict_code" = "409"
echo ok

printf 'second published version and rollback... '
preview3="$(curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$page_id/preview")"
test -n "$(printf '%s' "$preview3" | json_field preview_token)"
published2="$(curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$page_id/publish")"
printf '%s' "$published2" | grep -q 'DRAFT SECRET LEAK TEST'
rollback="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' \
  -d "{\"version_id\":\"$published1_id\"}" \
  "$BASE_URL/api/v1/cms/pages/$page_id/rollback")"
printf '%s' "$rollback" | grep -q "$published1_id"
rolled_public="$(curl -fsS "$BASE_URL/public/v1/cms/pages/ci-cms")"
printf '%s' "$rolled_public" | grep -q 'First Published Heading'
if printf '%s' "$rolled_public" | grep -q 'DRAFT SECRET LEAK TEST'; then exit 1; fi
echo ok

printf 'version history and CMS audit are complete... '
versions="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/cms/pages/$page_id/versions")"
printf '%s' "$versions" | python3 -c 'import json,sys; d=json.load(sys.stdin); nums=[x["version_no"] for x in d["items"]]; assert len(nums)>=7; assert len(nums)==len(set(nums)); assert any(x["rollback_of_version_id"] for x in d["items"])'
audit="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/cms/pages/$page_id/audit")"
printf '%s' "$audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); actions={x["action"] for x in d["items"]}; required={"PAGE_CREATED","DRAFT_SAVED","PREVIEW_CREATED","PUBLISHED","ROLLBACK_PUBLISHED"}; assert required.issubset(actions), (required,actions)'
echo ok

printf 'public manifest contains published content only... '
manifest="$(curl -fsS "$BASE_URL/public/v1/cms/manifest")"
printf '%s' "$manifest" | python3 -c 'import json,sys; d=json.load(sys.stdin); slugs={x["slug"] for x in d["items"]}; assert "ci-cms" in slugs; assert "ci-incomplete" not in slugs; assert "ci-conflict" not in slugs; forbidden={"page_id","page_key","name","created_by","published_by"}; assert all(not forbidden.intersection(x) for x in d["items"]), d'
echo ok

echo "HIMATE START-16 CMS integration smoke passed"
