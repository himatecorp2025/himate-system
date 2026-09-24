#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
EXPECTED_VERSION="${HIMATE_APP_VERSION:-0.8.30-start-23.11.5}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start23113j-owner.txt"
rm -f "$COOKIE"
trap 'rm -f "$COOKIE"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'release is synchronized before Golden Test acceptance... '
HEALTH="$(curl -fsS "$BASE_URL/api/v1/health")"
python3 - "$HEALTH" "$EXPECTED_VERSION" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); expected=sys.argv[2]
assert d["status"]=="ok",d
assert d["release_consistent"] is True,d
assert d["version"]==expected,d
PY
echo ok

STAMP="$(date +%s)"
TODAY="$(date -u +%Y-%m-%d)"
YEAR="$(date -u +%Y)"
METRIC_KEY="klavierhaus.events.attendance.attendee_count"
DISPLAY="Golden Test Partner $STAMP"

printf 'create ordinary PROSPECT partner... '
PARTNER_PAYLOAD="$(python3 - "$STAMP" "$DISPLAY" <<'PY'
import json,sys
stamp,display=sys.argv[1:]
print(json.dumps({
  "display_name":display,
  "legal_name":"Golden Test Partner LLC "+stamp,
  "brand_name":"Golden Test Brand "+stamp,
  "category_id":"cat_006",
  "lifecycle":"PROSPECT",
  "contact_name":"Golden Test Owner",
  "contact_email":"golden.test."+stamp+"@himate.test",
  "registration_number":"GOLDEN-"+stamp,
  "tax_id":"TEST-VAT-"+stamp,
  "country":"United States",
  "state_region":"New York",
  "city":"New York",
  "notes":"START-23.11.3j synthetic partner data",
  "onboarding_request_id":"golden-test-"+stamp
}))
PY
)"
PARTNER="$(curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$PARTNER_PAYLOAD" "$BASE_URL/api/v1/partners")"
PARTNER_ID="$(printf '%s' "$PARTNER" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["lifecycle"]=="PROSPECT"; assert d.get("test_partner") is False; print(d["id"])')"
echo ok

printf 'promote partner to Golden Test tenant without billing gate... '
PROMOTED="$(curl -fsS -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":true,"reason":"START-23.11.3j golden test activation"}' "$BASE_URL/api/v1/partners/$PARTNER_ID")"
printf '%s' "$PROMOTED" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["test_partner"] is True,d; assert d["lifecycle"]=="LIVE",d'
echo ok

printf 'Golden Test flag cannot be casually disabled... '
STATUS="$(curl -sS -o /tmp/himate-start23113j-disable.json -w '%{http_code}' -b "$COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"test_partner":false,"reason":"must be rejected"}' "$BASE_URL/api/v1/partners/$PARTNER_ID")"
test "$STATUS" = "409"
python3 - /tmp/himate-start23113j-disable.json <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
assert d["error"]["code"]=="GOLDEN_TEST_PARTNER_IMMUTABLE",d
PY
rm -f /tmp/himate-start23113j-disable.json
echo ok

printf 'Golden Test tenant exposes 38 canonical active modules... '
MODULES="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners/$PARTNER_ID/modules")"
python3 - "$MODULES" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
golden=[x for x in d["items"] if x.get("quote_reference")=="GOLDEN-TEST-PARTNER"]
assert len(golden)==38,(len(golden),[x.get("key") for x in golden])
for item in golden:
    assert item["status"]=="ACTIVE",item
    assert item["entitlement_state"]=="ACTIVE",item
    assert item["visible"] is True,item
    assert item["included_in_base"] is True,item
PY
echo ok

printf 'PostgreSQL stores exactly 38 canonical Golden Test entitlements... '
ACTIVE_COUNT="$(docker compose exec -T postgres psql -U himate -d himate -At -v partner_id="$PARTNER_ID" <<'SQL'
SELECT COUNT(*)
FROM catalog.partner_modules pm
JOIN catalog.modules m ON m.module_key=pm.module_key
WHERE pm.partner_id=:'partner_id'
  AND m.system=TRUE
  AND pm.status='ACTIVE'
  AND pm.entitlement_state='ACTIVE'
  AND pm.visible=TRUE
  AND pm.included_in_base=TRUE
  AND pm.entitlement_source='TEST'
  AND pm.plan_key='GOLDEN_TEST';
SQL
)"
test "$ACTIVE_COUNT" = "38"
echo ok

printf 'Golden Test flag and LIVE lifecycle persist in PostgreSQL... '
CORE_STATE="$(docker compose exec -T postgres psql -U himate -d himate -At -F '|' -v partner_id="$PARTNER_ID" <<'SQL'
SELECT test_partner,lifecycle FROM partners.partners WHERE id=:'partner_id';
SQL
)"
test "$CORE_STATE" = "t|LIVE"
echo ok

printf 'ensure authoritative People Reached metric exists... '
DEFINITIONS="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/impact/definitions")"
if ! printf '%s' "$DEFINITIONS" | python3 -c 'import json,sys; key=sys.argv[1]; raise SystemExit(0 if any(x["metric_key"]==key for x in json.load(sys.stdin)["items"]) else 1)' "$METRIC_KEY"; then
  METRIC_PAYLOAD="$(python3 - "$METRIC_KEY" <<'PY'
import json,sys
print(json.dumps({
  "metric_key":sys.argv[1],
  "label_en":"People Reached","label_hu":"Elért emberek",
  "description_en":"Authoritative digital attendance count",
  "description_hu":"Hiteles digitális látogatottsági darabszám",
  "unit":"count","aggregation":"SUM","scope":"PARTNER"
}))
PY
)"
  curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$METRIC_PAYLOAD" "$BASE_URL/api/v1/impact/definitions" >/dev/null
fi
echo ok

printf 'capture real platform Impact aggregate before synthetic test data... '
BEFORE="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
BEFORE_PEOPLE="$(printf '%s' "$BEFORE" | python3 -c 'import json,sys; print(float(json.load(sys.stdin)["impact"]["people_reached_ytd"]))')"
echo ok

printf 'record large synthetic Impact value on Golden Test partner... '
IMPACT_PAYLOAD="$(python3 - "$PARTNER_ID" "$METRIC_KEY" "$TODAY" "$STAMP" <<'PY'
import json,sys
partner,key,day,stamp=sys.argv[1:]
print(json.dumps({
  "partner_id":partner,
  "metric_key":key,
  "period_start":day,
  "period_end":day,
  "numeric_value":987654,
  "provenance":"MANUAL",
  "source_ref":"golden-test-"+stamp
}))
PY
)"
curl -fsS -b "$COOKIE" -H 'Content-Type: application/json' -d "$IMPACT_PAYLOAD" "$BASE_URL/api/v1/impact/values" >/dev/null
PARTNER_IMPACT="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/impact/summary?partner_id=$PARTNER_ID")"
printf '%s' "$PARTNER_IMPACT" | python3 -c 'import json,sys; d=json.load(sys.stdin); key=sys.argv[1]; x=next(i for i in d["items"] if i["metric_key"]==key); assert float(x["numeric_value"])==987654,x' "$METRIC_KEY"
echo ok

printf 'synthetic Golden Test Impact stays out of HIMATE platform aggregate... '
AFTER="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/dashboard/summary?year=$YEAR&refresh=true")"
printf '%s' "$AFTER" | python3 -c 'import json,sys; after=float(json.load(sys.stdin)["impact"]["people_reached_ytd"]); before=float(sys.argv[1]); assert abs(after-before)<0.000001,(before,after)' "$BEFORE_PEOPLE"
echo ok

echo 'HIMATE START-23.11.3j Golden Test Partner & Workspace Stabilization smoke passed'
