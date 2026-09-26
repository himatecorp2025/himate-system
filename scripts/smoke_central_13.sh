#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-central13-owner.txt"
HEADERS="$TMP_ROOT/himate-central13-headers.txt"
BODY="$TMP_ROOT/himate-central13-body.txt"
rm -f "$COOKIE" "$HEADERS" "$BODY"
trap 'rm -f "$COOKIE" "$HEADERS" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
STAMP="$(date +%s)"

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'CENTRAL-13 health gate... '
curl -fsS "$BASE_URL/api/v1/health" | python3 -c 'import json,sys; assert json.load(sys.stdin)["status"]=="ok"'
echo ok

printf 'save expanded Design Guide controls to authoritative CMS draft... '
design_before="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/design")"
design_payload="$(printf '%s' "$design_before" | python3 -c '
import json,sys
d=json.load(sys.stdin)["draft"]
d["layout_key"]="modern_grid"
d["navy"]="#123A63"
d["gold"]="#C49A55"
d["background"]="#F5F1E8"
d["text_color"]="#263448"
d["heading_font"]="Palatino"
d["body_font"]="Trebuchet MS"
d["button_radius"]=14
print(json.dumps(d,separators=(",",":")))
')"
curl -fsS -b "$COOKIE" -X PUT -H 'Content-Type: application/json' -d "$design_payload" "$BASE_URL/api/v1/cms/design/draft" >/dev/null
design_read="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/design")"
printf '%s' "$design_read" | python3 -c '
import json,sys
d=json.load(sys.stdin)["draft"]
assert d["layout_key"]=="modern_grid",d
assert d["heading_font"]=="Palatino",d
assert d["body_font"]=="Trebuchet MS",d
assert d["navy"]=="#123A63",d
assert d["gold"]=="#C49A55",d
'
echo ok

printf 'real Desktop / Tablet / Mobile Design preview uses exact viewport frames... '
preview="$(curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/design/preview")"
for spec in "desktop_path|1440px|900px" "tablet_path|834px|1194px" "mobile_path|390px|844px"; do
  key="$(printf '%s' "$spec" | cut -d'|' -f1)"
  width="$(printf '%s' "$spec" | cut -d'|' -f2)"
  height="$(printf '%s' "$spec" | cut -d'|' -f3)"
  path="$(printf '%s' "$preview" | python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$key")"
  curl -fsS -D "$HEADERS" "$BASE_URL$path" -o "$BODY"
  grep -qi '^X-Himate-Preview: design' "$HEADERS"
  grep -qi '^X-Robots-Tag: noindex, nofollow' "$HEADERS"
  grep -q "width:$width" "$BODY"
  grep -q "height:$height" "$BODY"
done
echo ok

printf 'preview token state is truthful and raw preview renders layout + typography... '
design_after_preview="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/cms/design")"
printf '%s' "$design_after_preview" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["preview_active"] is True,d'
desktop_path="$(printf '%s' "$preview" | python3 -c 'import json,sys; print(json.load(sys.stdin)["desktop_path"])')"
curl -fsS "$BASE_URL$desktop_path&raw=1" -o "$BODY"
grep -q 'data-himate-design' "$BODY"
grep -q 'data-layout="modern_grid"' "$BODY"
grep -q 'grid-template-columns:repeat(12' "$BODY"
grep -q 'Palatino' "$BODY"
grep -q 'Trebuchet MS' "$BODY"
grep -q -- '--navy:#123A63' "$BODY"
echo ok

printf 'Publish Active Brand makes the same design authoritative on the public website... '
published="$(curl -fsS -b "$COOKIE" -X POST "$BASE_URL/api/v1/cms/design/publish")"
printf '%s' "$published" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["version"]>=1,d; assert d["published_at"],d'
curl -fsS -D "$HEADERS" "$BASE_URL/" -o "$BODY"
grep -qi '^X-Himate-Design: published' "$HEADERS"
grep -q 'data-layout="modern_grid"' "$BODY"
grep -q 'Palatino' "$BODY"
grep -q 'Trebuchet MS' "$BODY"
echo ok

printf 'create CENTRAL-13 partner for real connection aggregation... '
PARTNER_NAME="Central 13 Connection Partner $STAMP"
partner_payload="$(python3 - "$STAMP" "$PARTNER_NAME" <<'PY'
import json,sys
stamp,name=sys.argv[1:]
print(json.dumps({
  "display_name":name,
  "legal_name":"Central 13 Connection Partner LLC "+stamp,
  "brand_name":"Central 13 Runtime "+stamp,
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "contact_name":"Central 13 QA",
  "contact_email":"central13."+stamp+"@himate.test",
  "country":"United States",
  "state_region":"New York",
  "city":"New York",
  "notes":"CENTRAL-13 partner connections runtime fixture"
}))
PY
)"
partner="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
PARTNER_ID="$(printf '%s' "$partner" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
test -n "$PARTNER_ID"
echo ok

printf 'seed real Connector credential/state and Website Adapter records... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -v partner_id="$PARTNER_ID" -v stamp="$STAMP" <<'SQL' >/dev/null
INSERT INTO connector.credentials(partner_id,environment,credential_id,token_hash,active,last_used_at)
VALUES(:'partner_id','PRODUCTION','central13-cred-'||:'stamp','central13-hash-'||:'stamp',TRUE,NOW())
ON CONFLICT(partner_id,environment) DO UPDATE
SET active=TRUE,last_used_at=NOW(),rotated_at=NOW();

INSERT INTO connector.partner_state(
  partner_id,environment,reported_version,health,module_state,last_seen_at,last_error,
  protocol_version,sync_status,last_data_sync_at,last_reconciliation_at,updated_at)
VALUES(
  :'partner_id','PRODUCTION','central13-runtime','OK','{}'::jsonb,NOW(),'',
  'START-22','SYNCED',NOW(),NOW(),NOW())
ON CONFLICT(partner_id,environment) DO UPDATE
SET health='OK',sync_status='SYNCED',last_error='',last_data_sync_at=NOW(),
    last_reconciliation_at=NOW(),updated_at=NOW();

INSERT INTO connector.website_adapters(
  partner_id,environment,adapter_type,site_base_url,allowed_domains,capabilities,
  privacy_mode,enabled,config,updated_by,updated_at)
VALUES(
  :'partner_id','PRODUCTION','GENERIC_HTTP','https://central13.example.test',
  '["central13.example.test"]'::jsonb,'["ENTITLEMENTS","METRICS"]'::jsonb,
  'AGGREGATED_ONLY',TRUE,'{}'::jsonb,'central13-ci',NOW())
ON CONFLICT(partner_id,environment) DO UPDATE
SET adapter_type='GENERIC_HTTP',site_base_url=EXCLUDED.site_base_url,enabled=TRUE,updated_at=NOW();
SQL
echo ok

CONNECTION_URL="$BASE_URL/api/v1/central/connections?q=$PARTNER_ID&limit=120&offset=0"
printf 'partner-first Connections model returns real integrations and last successful sync... '
curl -fsS -D "$HEADERS" -b "$COOKIE" "$CONNECTION_URL" -o "$BODY"
python3 - "$BODY" "$PARTNER_ID" "$PARTNER_NAME" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
pid,name=sys.argv[2:]
rows=[x for x in d["items"] if x["partner_id"]==pid]
assert len(rows)==1,(pid,d)
row=rows[0]
assert row["partner_name"]==name,row
assert row["connection_status"]=="ACTIVE",row
assert row["last_successful_sync"],row
assert row["last_error"]=="",row
assert row["integration_count"]==2,row
assert set(row["connection_types"])=={"DATA_CONNECTOR","GENERIC_HTTP"},row
assert all(x["type"]!="KLAVIYO" for x in row["integrations"]),row
assert d["meta"]["source"]=="PARTNERS_CONNECTOR_RUNTIME_WEBSITE_ADAPTERS",d
PY
echo ok

printf 'same Connections key is cache-backed after the first read... '
curl -fsS -D "$HEADERS" -b "$COOKIE" "$CONNECTION_URL" -o "$BODY"
grep -qi '^X-Himate-Cache: hit' "$HEADERS"
echo ok

printf 'runtime failure derives SUSPENDED and exposes the actual last error... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -v partner_id="$PARTNER_ID" <<'SQL' >/dev/null
UPDATE connector.partner_state
SET health='OFFLINE',sync_status='FAILED',last_error='CENTRAL-13 synthetic runtime outage',updated_at=NOW()
WHERE partner_id=:'partner_id' AND environment='PRODUCTION';
SQL
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/connections?q=$PARTNER_ID&status=SUSPENDED&limit=120&offset=0" | python3 -c '
import json,sys
d=json.load(sys.stdin); assert len(d["items"])==1,d
row=d["items"][0]
assert row["connection_status"]=="SUSPENDED",row
assert row["last_error"]=="CENTRAL-13 synthetic runtime outage",row
'
echo ok

printf 'authoritative archived lifecycle derives DELETED at the partner layer... '
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -v partner_id="$PARTNER_ID" <<'SQL' >/dev/null
UPDATE partners.partners SET lifecycle='ARCHIVED',updated_at=NOW() WHERE id=:'partner_id';
SQL
curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/central/connections?q=$PARTNER_ID&status=DELETED&limit=120&offset=0" | python3 -c '
import json,sys
d=json.load(sys.stdin); assert len(d["items"])==1,d
assert d["items"][0]["connection_status"]=="DELETED",d["items"][0]
'
echo ok

echo 'CENTRAL-13 Website, Marketing & Partner Operations runtime acceptance passed'
