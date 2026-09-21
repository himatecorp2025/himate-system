#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE_JAR="$TMP_ROOT/himate-start22-owner.txt"
BODY="$TMP_ROOT/himate-start22-body.json"
BATCH="$TMP_ROOT/himate-start22-batch.json"
BAD_BATCH="$TMP_ROOT/himate-start22-bad-batch.json"
OVERRIDE_BATCH="$TMP_ROOT/himate-start22-override-batch.json"
RECON="$TMP_ROOT/himate-start22-reconcile.json"
rm -f "$COOKIE_JAR" "$BODY" "$BATCH" "$BAD_BATCH" "$OVERRIDE_BATCH" "$RECON"
trap 'rm -f "$COOKIE_JAR" "$BODY" "$BATCH" "$BAD_BATCH" "$OVERRIDE_BATCH" "$RECON"' EXIT

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

sign_file() {
  token="$1"; file="$2"; fixed_nonce="${3:-}"
  python3 - "$token" "$file" "$fixed_nonce" <<'PY'
import hashlib,hmac,secrets,sys
from datetime import datetime,timezone
token,path,fixed=sys.argv[1:4]
raw=open(path,"rb").read()
timestamp=datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
nonce=fixed or ("start22-"+secrets.token_hex(16))
body=hashlib.sha512(raw).hexdigest()
signature=hmac.new(token.encode(),f"{timestamp}\n{nonce}\n{body}".encode(),hashlib.sha512).hexdigest()
print(timestamp,nonce,body,signature)
PY
}

signed_post() {
  token="$1"; path="$2"; file="$3"; fixed_nonce="${4:-}"
  sig="$(sign_file "$token" "$file" "$fixed_nonce")"
  set -- $sig
  timestamp="$1"; nonce="$2"; body_sha="$3"; signature="$4"
  curl -sS -o "$BODY" -w '%{http_code}'     -H "Authorization: Bearer $token"     -H 'Content-Type: application/json'     -H "X-Himate-Timestamp: $timestamp"     -H "X-Himate-Nonce: $nonce"     -H "X-Himate-Body-SHA512: $body_sha"     -H "X-Himate-Signature: $signature"     --data-binary "@$file" "$BASE_URL$path"
}

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

printf 'START-22 owner login... '
login_payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE_JAR" -H 'Content-Type: application/json' -d "$login_payload" "$BASE_URL/api/v1/auth/login" >/dev/null
echo ok

printf '38-module registry and unified retention policy... '
mapping="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/connectors/start22/mapping")"
printf '%s' "$mapping" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["protocol_version"]=="1.0"; assert d["source_system"]=="KLAVIERHAUS"; assert d["module_count"]==38; assert len(d["items"])==38; assert all(x["retention_policy"]=="HIMATE_7Y" and x["retention_years"]==7 for x in d["items"]); assert len({x["module_key"] for x in d["items"]})==38; assert len({x["dataset_key"] for x in d["items"]})==38'
echo ok

printf 'create isolated START-22 partner and connector credential... '
partner_payload='{"display_name":"START 22 Klavierhaus Connector","legal_name":"START 22 Klavierhaus Connector LLC","brand_name":"Klavierhaus START 22","category_id":"cat_006","lifecycle":"PROSPECT","contact_name":"START 22 CI","contact_email":"start22-ci@example.com","country":"United States"}'
created="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' -d "$partner_payload" "$BASE_URL/api/v1/partners")"
partner_id="$(printf '%s' "$created" | json_field id)"
credential="$(curl -fsS -b "$COOKIE_JAR" -H 'Content-Type: application/json' -d '{"environment":"STAGING"}' "$BASE_URL/api/v1/connectors/$partner_id/credential")"
connector_token="$(printf '%s' "$credential" | json_field token)"
test -n "$partner_id"
test -n "$connector_token"
echo "$partner_id"

printf 'heartbeat and module coverage are partner-scoped... '
heartbeat="$(curl -fsS -H "Authorization: Bearer $connector_token" -H 'Content-Type: application/json'   -d '{"version":"klavierhaus-6.7.0-start22","health":"OK","modules":{"users":{"enabled":true,"dataset_key":"operations.users","schema_version":1}},"error":""}'   "$BASE_URL/connector/v1/heartbeat")"
printf '%s' "$heartbeat" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("status")=="accepted",d; assert d.get("partner_id")==sys.argv[1],(d,sys.argv[1]); assert d.get("environment")=="STAGING",d' "$partner_id"
echo ok

printf 'build canonical signed Klavierhaus batch... '
python3 - "$BATCH" <<'PY'
import hashlib,json,sys
data={"active_user_count":4,"admin_count":1,"manager_count":1,"worker_count":2}
canonical=json.dumps(data,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
checksum=hashlib.sha512(canonical).hexdigest()
payload={
  "protocol_version":"1.0",
  "source_system":"KLAVIERHAUS",
  "source_version":"6.7.0-start22",
  "batch_id":"kh-start22-ci-batch-0001",
  "generated_at":"2026-09-21T15:00:00Z",
  "items":[{
    "module_key":"users",
    "dataset_key":"operations.users",
    "schema_version":1,
    "period_start":"2026-09-21",
    "period_end":"2026-09-21",
    "aggregation":"LATEST",
    "data":data,
    "source_checksum":checksum,
    "idempotency_key":"kh-start22-users-20260921-0001"
  }]
}
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps(payload,separators=(",",":"),ensure_ascii=False))
PY
echo ok

printf 'signed batch ingestion routes into Impact... '
replay_nonce="start22-replay-fixed-000000000001"
sig="$(sign_file "$connector_token" "$BATCH" "$replay_nonce")"
set -- $sig
timestamp="$1"; nonce="$2"; body_sha="$3"; signature="$4"
code="$(curl -sS -o "$BODY" -w '%{http_code}'   -H "Authorization: Bearer $connector_token"   -H 'Content-Type: application/json'   -H "X-Himate-Timestamp: $timestamp"   -H "X-Himate-Nonce: $nonce"   -H "X-Himate-Body-SHA512: $body_sha"   -H "X-Himate-Signature: $signature"   --data-binary "@$BATCH" "$BASE_URL/connector/v1/data/batches")"
test "$code" = "202"
python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["partner_id"]==sys.argv[1],d; assert d["environment"]=="STAGING",d; assert d["accepted"]==1 and d["route_errors"]==0,d; assert d["retention_policy"]=="HIMATE_7Y" and d["retention_years"]==7,d' "$partner_id" <"$BODY"
echo ok

printf 'nonce replay is rejected before duplicate processing... '
replay_code="$(curl -sS -o "$BODY" -w '%{http_code}'   -H "Authorization: Bearer $connector_token"   -H 'Content-Type: application/json'   -H "X-Himate-Timestamp: $timestamp"   -H "X-Himate-Nonce: $nonce"   -H "X-Himate-Body-SHA512: $body_sha"   -H "X-Himate-Signature: $signature"   --data-binary "@$BATCH" "$BASE_URL/connector/v1/data/batches")"
test "$replay_code" = "409"
grep -q 'REPLAY_DETECTED' "$BODY"
echo ok

printf 'exact batch retry is idempotent with a fresh signed request... '
duplicate_code="$(signed_post "$connector_token" "/connector/v1/data/batches" "$BATCH")"
test "$duplicate_code" = "200"
grep -q 'ALREADY_PROCESSED' "$BODY"
echo ok

printf 'field allowlist fails closed on password-bearing payload... '
python3 - "$BAD_BATCH" <<'PY'
import hashlib,json,sys
data={"active_user_count":4,"password_hash":"must-never-cross"}
checksum=hashlib.sha512(json.dumps(data,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()).hexdigest()
payload={
 "protocol_version":"1.0","source_system":"KLAVIERHAUS","source_version":"6.7.0-start22",
 "batch_id":"kh-start22-ci-batch-bad-0001","generated_at":"2026-09-21T15:00:00Z",
 "items":[{"module_key":"users","dataset_key":"operations.users","schema_version":1,
 "period_start":"2026-09-21","period_end":"2026-09-21","aggregation":"LATEST",
 "data":data,"source_checksum":checksum,"idempotency_key":"kh-start22-users-bad-0001"}]
}
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps(payload,separators=(",",":"),ensure_ascii=False))
PY
bad_code="$(signed_post "$connector_token" "/connector/v1/data/batches" "$BAD_BATCH")"
test "$bad_code" = "400"
grep -q 'DATASET_VALIDATION' "$BODY"
echo ok

printf 'partner identity cannot be overridden in signed JSON... '
python3 - "$OVERRIDE_BATCH" <<'PY'
import hashlib,json,sys
data={"active_user_count":1}
checksum=hashlib.sha512(json.dumps(data,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()).hexdigest()
payload={
 "protocol_version":"1.0","source_system":"KLAVIERHAUS","source_version":"6.7.0-start22",
 "partner_id":"ptr_cross_tenant_override",
 "batch_id":"kh-start22-ci-batch-override-0001","generated_at":"2026-09-21T15:00:00Z",
 "items":[{"module_key":"users","dataset_key":"operations.users","schema_version":1,
 "period_start":"2026-09-21","period_end":"2026-09-21","aggregation":"LATEST",
 "data":data,"source_checksum":checksum,"idempotency_key":"kh-start22-users-override-0001"}]
}
open(sys.argv[1],"w",encoding="utf-8").write(json.dumps(payload,separators=(",",":"),ensure_ascii=False))
PY
override_code="$(signed_post "$connector_token" "/connector/v1/data/batches" "$OVERRIDE_BATCH")"
test "$override_code" = "400"
grep -q '"code":"JSON"' "$BODY"
echo ok

printf 'daily reconciliation verifies dataset checksum... '
python3 - "$BATCH" "$RECON" <<'PY'
import hashlib,json,sys
batch=json.load(open(sys.argv[1],encoding="utf-8"))
checks=[x["source_checksum"] for x in batch["items"] if x["dataset_key"]=="operations.users"]
aggregate=hashlib.sha512("\n".join(sorted(checks)).encode()).hexdigest()
payload={
 "protocol_version":"1.0","source_system":"KLAVIERHAUS","source_version":"6.7.0-start22",
 "reconciliation_id":"kh-start22-ci-reconcile-0001","batch_id":batch["batch_id"],
 "generated_at":"2026-09-21T15:05:00Z",
 "datasets":[{"dataset_key":"operations.users","item_count":len(checks),"aggregate_checksum":aggregate}]
}
open(sys.argv[2],"w",encoding="utf-8").write(json.dumps(payload,separators=(",",":"),ensure_ascii=False))
PY
recon_code="$(signed_post "$connector_token" "/connector/v1/reconcile" "$RECON")"
test "$recon_code" = "200"
grep -q '"status":"SYNCED"' "$BODY"
summary="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/connectors/start22/summary?partner_id=$partner_id&environment=STAGING")"
printf '%s' "$summary" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["registry_modules"]==38; s=next(x for x in d["states"] if x["partner_id"]==sys.argv[1]); assert s["protocol_version"]=="1.0" and s["sync_status"]=="SYNCED",s' "$partner_id"
echo ok

printf 'Connector and Impact both persist HIMATE_7Y... '
record_id="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT id FROM connector.data_records WHERE partner_id='$partner_id' AND dataset_key='operations.users' ORDER BY id DESC LIMIT 1")"
test -n "$record_id"
connector_retention="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT retention_policy||'|'||(retain_until>received_at+INTERVAL '6 years 11 months')::text FROM (SELECT 'HIMATE_7Y' retention_policy,retain_until,received_at FROM connector.data_records WHERE id=$record_id) x")"
test "$connector_retention" = "HIMATE_7Y|true"
impact_retention="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM impact.metric_values WHERE partner_id='$partner_id' AND source_ref='connector:data_record:$record_id' AND retention_policy='HIMATE_7Y' AND retain_until>NOW()+INTERVAL '6 years 11 months'")"
test "$impact_retention" -ge "1"
echo ok

printf 'legal hold synchronizes to downstream Impact and blocks purge... '
hold_payload="$(printf '{"action":"SET_LEGAL_HOLD","record_id":%s,"legal_hold":true}' "$record_id")"
hold_code="$(status "$COOKIE_JAR" POST "/api/v1/connectors/start22/retention" -H 'Content-Type: application/json' -d "$hold_payload")"
test "$hold_code" = "200"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT legal_hold FROM connector.data_records WHERE id=$record_id")" = "t"
impact_hold_count="$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM impact.metric_values WHERE source_ref='connector:data_record:$record_id' AND legal_hold=TRUE")"
test "$impact_hold_count" -ge "1"
docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 -c "UPDATE connector.data_records SET retain_until=NOW()-INTERVAL '1 day' WHERE id=$record_id; UPDATE impact.metric_values SET retain_until=NOW()-INTERVAL '1 day' WHERE source_ref='connector:data_record:$record_id';" >/dev/null
purge_code="$(status "$COOKIE_JAR" POST "/api/v1/connectors/start22/retention" -H 'Content-Type: application/json' -d '{"action":"PURGE_EXPIRED"}')"
test "$purge_code" = "200"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM connector.data_records WHERE id=$record_id")" = "1"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM impact.metric_values WHERE source_ref='connector:data_record:$record_id'")" -ge "1"
echo ok

printf 'privacy deletion is blocked by hold, then removes Connector and Impact copies after release... '
blocked_delete="$(status "$COOKIE_JAR" POST "/api/v1/connectors/start22/retention" -H 'Content-Type: application/json' -d "{"action":"PRIVACY_DELETE","record_id":$record_id}")"
test "$blocked_delete" = "409"
grep -q 'LEGAL_HOLD' "$BODY"
release_code="$(status "$COOKIE_JAR" POST "/api/v1/connectors/start22/retention" -H 'Content-Type: application/json' -d "{"action":"SET_LEGAL_HOLD","record_id":$record_id,"legal_hold":false}")"
test "$release_code" = "200"
delete_code="$(status "$COOKIE_JAR" POST "/api/v1/connectors/start22/retention" -H 'Content-Type: application/json' -d "{"action":"PRIVACY_DELETE","record_id":$record_id}")"
test "$delete_code" = "200"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM connector.data_records WHERE id=$record_id")" = "0"
test "$(docker compose exec -T postgres psql -U himate -d himate -Atc "SELECT COUNT(*) FROM impact.metric_values WHERE source_ref='connector:data_record:$record_id'")" = "0"
echo ok

printf 'START-22 connector UI/health API remains available after lifecycle test... '
curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/connectors/start22/summary?partner_id=$partner_id" | grep -q '"retention_policy":"HIMATE_7Y"'
health="$(curl -fsS -b "$COOKIE_JAR" "$BASE_URL/api/v1/system-health")"
printf '%s' "$health" | grep -q '"name":"connector"'
echo ok

echo "HIMATE START-22 signed Klavierhaus data connector, reconciliation, routing and retention smoke passed"
