#!/usr/bin/env sh
set -eu

if [ "$#" -gt 0 ]; then BASE_URL="$1"; else BASE_URL="http://127.0.0.1:8080"; fi
COOKIE_JAR="/tmp/himate-start-17-cookies.txt"
BODY="/tmp/himate-start-17-body.txt"
HEADERS="/tmp/himate-start-17-headers.txt"
rm -f "$COOKIE_JAR" "$BODY" "$HEADERS"
trap 'rm -f "$COOKIE_JAR" "$BODY" "$HEADERS"' EXIT

printf 'login... '
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"email":"admin@example.com","password":"Local-Development1!Password"}'   "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf 'create and publish CMS-backed platform page... '
page="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json'   -d '{"page_key":"ci_ssr_platform","name":"SSR Platform","version":{"slug":"platform","seo":{"title":"SSR Platform Title","meta_description":"Server-rendered metadata from HIMATE CMS.","canonical":"https://www.himate.com/platform","og_title":"SSR Platform OG","og_description":"SSR Open Graph description","og_image_asset_id":"","noindex":false},"sections":[{"id":"hero","component_type":"HERO","heading":"SERVER RENDERED PLATFORM","body":"This body must exist in the initial HTML response.","media_asset_id":"","cta_label":"See modules","cta_url":"/modules","visible":true,"sort_order":10,"settings":{}},{"id":"primary","component_type":"TEXT","heading":"Primary from CMS","body":"Primary CMS body","media_asset_id":"","cta_label":"","cta_url":"","visible":false,"sort_order":20,"settings":{}}]}}'   "$BASE_URL/api/v1/cms/pages")"
page_id="$(printf '%s' "$page" | python3 -c 'import json,sys; print(json.load(sys.stdin)["page"]["id"])')"
curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$page_id/preview" >/dev/null
curl -fsS -b "$COOKIE_JAR" -X POST "$BASE_URL/api/v1/cms/pages/$page_id/publish" >/dev/null
echo ok

printf 'public HTML is server-rendered before JavaScript... '
curl -fsS -D "$HEADERS" "$BASE_URL/platform" -o "$BODY"
grep -qi '^X-Himate-SSR: published' "$HEADERS"
grep -q '<title>SSR Platform Title</title>' "$BODY"
grep -q 'name="description" content="Server-rendered metadata from HIMATE CMS."' "$BODY"
grep -q 'rel="canonical" href="https://www.himate.com/platform"' "$BODY"
grep -q 'property="og:title" content="SSR Platform OG"' "$BODY"
grep -q 'SERVER RENDERED PLATFORM' "$BODY"
grep -q 'This body must exist in the initial HTML response.' "$BODY"
if grep -q 'Primary from CMS' "$BODY"; then exit 1; fi
echo ok

printf 'sitemap and robots are CMS-backed... '
curl -fsS "$BASE_URL/sitemap.xml" -o "$BODY"
grep -q 'https://www.himate.com/platform' "$BODY"
curl -fsS "$BASE_URL/robots.txt" -o "$BODY"
grep -q 'Sitemap: ' "$BODY"
echo ok

echo "HIMATE START-17 public SSR and SEO smoke passed"
