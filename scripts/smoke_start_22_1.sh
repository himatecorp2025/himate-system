#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
OWNER_COOKIE="$TMP_ROOT/himate-221-owner.txt"
USER_COOKIE="$TMP_ROOT/himate-221-user.txt"
BODY="$TMP_ROOT/himate-221-body.json"
rm -f "$OWNER_COOKIE" "$USER_COOKIE" "$BODY"
trap 'rm -f "$OWNER_COOKIE" "$USER_COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"
USER_PASSWORD="$(python3 -c 'import secrets; print("Cc3!"+secrets.token_urlsafe(24))')"

login() {
  cookie="$1"; email="$2"; password="$3"
  payload="$(python3 - "$email" "$password" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
  curl -fsS -c "$cookie" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login"
}

status() {
  cookie="$1"; method="$2"; path="$3"; shift 3
  curl -sS -o "$BODY" -w '%{http_code}' -b "$cookie" -X "$method" "$@" "$BASE_URL$path"
}

printf 'owner login and notification service health... '
login "$OWNER_COOKIE" "$OWNER_EMAIL" "$OWNER_PASSWORD" >/dev/null
health="$(curl -fsS "$BASE_URL/api/v1/health")"
printf '%s' "$health" | python3 -c 'import json,sys; d=json.load(sys.stdin); s=d.get("services",{}); assert s.get("notifications")=="ok",s'
echo ok

printf 'global SEO settings and audit endpoints are reachable... '
seo="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/cms/seo")"
printf '%s' "$seo" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d,dict)'
seo_audit="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/cms/seo/audit")"
printf '%s' "$seo_audit" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d,dict)'
echo ok

printf 'create and update custom permission role... '
role_payload='{"key":"ci_module_viewer","label":"CI Module Viewer","description":"START-22.1 scoped role","permissions":["catalog.read","notifications.read"]}'
role="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$role_payload" "$BASE_URL/api/v1/admin/roles")"
printf '%s' "$role" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["key"]=="ci_module_viewer"; assert d["system"] is False; assert set(d["permissions"])=={"catalog.read","notifications.read"}'
updated_role="$(curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json' -d '{"label":"CI Module Viewer Updated","permissions":["catalog.read","notifications.read"],"active":true}' "$BASE_URL/api/v1/admin/roles/ci_module_viewer")"
printf '%s' "$updated_role" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["label"]=="CI Module Viewer Updated"'
echo ok

printf 'custom role can be assigned and is backend enforced... '
user_payload="$(python3 - "$USER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"name":"START 22.1 Viewer","email":"ci-start-221-viewer@example.com","password":sys.argv[1],"roles":["ci_module_viewer"]}))
PY
)"
created_user="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json' -d "$user_payload" "$BASE_URL/api/v1/admin/users")"
printf '%s' "$created_user" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["roles"]==["ci_module_viewer"]; assert set(d["permissions"])=={"catalog.read","notifications.read"}'
login "$USER_COOKIE" "ci-start-221-viewer@example.com" "$USER_PASSWORD" >/dev/null
curl -fsS -b "$USER_COOKIE" "$BASE_URL/api/v1/modules" >/dev/null
test "$(status "$USER_COOKIE" GET "/api/v1/billing/profile")" = "403"
test "$(status "$OWNER_COOKIE" PATCH "/api/v1/admin/roles/ci_module_viewer" -H 'Content-Type: application/json' -d '{"active":false}')" = "409"
grep -q 'ROLE_IN_USE' "$BODY"
echo ok

printf 'create module group and technical registry module... '
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"group_key":"ci_221","label":"CI START 22.1","sort_order":90}'   "$BASE_URL/api/v1/module-groups" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X PATCH -H 'Content-Type: application/json'   -d '{"label":"CI START 22.1 Modules","sort_order":91}'   "$BASE_URL/api/v1/module-groups/ci_221" >/dev/null
module="$(curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"key":"ci.control_plane","label":"CI Control Plane Module","group_key":"ci_221","description":"Acceptance module","currency":"USD","version":"1.0.0","latest_version":"1.0.0","default_monthly_price":199,"availability":"ACTIVE","module_type":"INTEGRATION","owner_team":"Platform","source_repository":"himatecorp2025/himate-system","source_path":"modules/ci/control-plane","source_ref":"main","source_commit":"0123456789abcdef","artifact_type":"docker","artifact_reference":"sha256:ci221","min_platform_version":"0.8.1","manifest":{"schema_version":1}}'   "$BASE_URL/api/v1/modules")"
printf '%s' "$module" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["key"]=="ci.control_plane"; assert d["module_type"]=="INTEGRATION"'
catalog="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules")"
printf '%s' "$catalog" | python3 -c 'import json,sys; d=json.load(sys.stdin); m=next(x for x in d["items"] if x["key"]=="ci.control_plane"); assert m["source_repository"]=="himatecorp2025/himate-system"; assert m["source_path"]=="modules/ci/control-plane"; assert m["artifact_reference"]=="sha256:ci221"'
echo ok

printf 'module relationship, impact mapping and partner usage endpoints... '
curl -fsS -b "$OWNER_COOKIE" -H 'Content-Type: application/json'   -d '{"target_module_key":"contacts","relation_type":"REQUIRES","note":"CI dependency"}'   "$BASE_URL/api/v1/modules/ci.control_plane/relationships" >/dev/null
relationships="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules/ci.control_plane/relationships")"
printf '%s' "$relationships" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert any(x["target_module_key"]=="contacts" and x["relation_type"]=="REQUIRES" for x in d["items"])'
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json'   -d '{"items":[{"metric_key":"ci.module.usage","label":"CI module usage"}]}'   "$BASE_URL/api/v1/modules/ci.control_plane/impact-metrics" >/dev/null
metrics="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules/ci.control_plane/impact-metrics")"
printf '%s' "$metrics" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["items"][0]["metric_key"]=="ci.module.usage"'
usage="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/modules/ci.control_plane/usage")"
printf '%s' "$usage" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "items" in d and "status_counts" in d'
echo ok

printf 'HIMATE company profile remains authoritative and editable... '
company="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/billing/profile")"
company_write="$(printf '%s' "$company" | python3 -c '
import json,sys
d=json.load(sys.stdin)
keys=["legal_name","registration_number","address","tax_id","contact_name","email","phone","bank_name","bank_address","account_number","iban","swift","vat_rate_percent","vat_jurisdiction","tax_label"]
print(json.dumps({k:d.get(k) for k in keys}))
')"
company_payload="$(printf '%s' "$company_write" | python3 -c 'import json,sys; d=json.load(sys.stdin); d["contact_name"]="START-22.1 Acceptance"; print(json.dumps(d))')"
company_updated="$(curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$company_payload" "$BASE_URL/api/v1/billing/profile")"
printf '%s' "$company_updated" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["contact_name"]=="START-22.1 Acceptance"; assert "vat_enabled" in d'
curl -fsS -b "$OWNER_COOKIE" -X PUT -H 'Content-Type: application/json' -d "$company_write" "$BASE_URL/api/v1/billing/profile" >/dev/null
echo ok

printf 'notification center receives audited control-plane events... '
sleep 2
notifications="$(curl -fsS -b "$OWNER_COOKIE" "$BASE_URL/api/v1/notifications?limit=100")"
notification_id="$(printf '%s' "$notifications" | python3 -c 'import json,sys; d=json.load(sys.stdin); x=next(x for x in d["items"] if x["event_type"]=="MODULE_CREATED"); print(x["id"])')"
test -n "$notification_id"
curl -fsS -b "$OWNER_COOKIE" -X POST "$BASE_URL/api/v1/notifications/$notification_id/read" >/dev/null
curl -fsS -b "$OWNER_COOKIE" -X POST "$BASE_URL/api/v1/notifications/read-all" >/dev/null
echo ok

echo "HIMATE START-22.1 Control Plane Completion smoke passed"
