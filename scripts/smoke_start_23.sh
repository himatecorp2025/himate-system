#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
TMP_ROOT="${TMPDIR:-/tmp}"
COOKIE="$TMP_ROOT/himate-start23-owner.txt"
BODY="$TMP_ROOT/himate-start23-body.json"
rm -f "$COOKIE" "$BODY"
trap 'rm -f "$COOKIE" "$BODY"' EXIT

COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

payload="$(python3 - "$OWNER_EMAIL" "$OWNER_PASSWORD" <<'PY'
import json,sys
print(json.dumps({"email":sys.argv[1],"password":sys.argv[2],"remember":False}))
PY
)"
curl -fsS -c "$COOKIE" -H 'Content-Type: application/json' -d "$payload" "$BASE_URL/api/v1/auth/login" >/dev/null

printf 'START-23 public surfaces... '
curl -fsS "$BASE_URL/" >/dev/null
curl -fsS "$BASE_URL/partner/login" >/dev/null
echo ok

printf 'START-23 authenticated workspace route matrix... '
for path in   '/api/v1/dashboard/summary'   '/api/v1/partners?limit=5&offset=0'   '/api/v1/modules?limit=5&offset=0'   '/api/v1/billing/profile'   '/api/v1/impact/summary'   '/api/v1/evidence?limit=5&offset=0'   '/api/v1/reports?limit=5&offset=0'   '/api/v1/cms/pages?limit=5&offset=0'   '/api/v1/cms/media?limit=5&offset=0'   '/api/v1/cms/seo'   '/api/v1/contact/inquiries?limit=5&offset=0'   '/api/v1/system-health/snapshot'   '/api/v1/provisioning/jobs?limit=5&offset=0'   '/api/v1/environments?limit=5&offset=0'   '/api/v1/backups/summary'   '/api/v1/connectors/start22/summary'   '/api/v1/admin/roles'   '/api/v1/admin/users'   '/api/v1/audit/events?limit=5&offset=0'   '/api/v1/notifications?limit=5'
do
  code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE" "$BASE_URL$path")"
  if [ "$code" != "200" ]; then
    echo "route failed: $path status=$code" >&2
    cat "$BODY" >&2
    exit 1
  fi
done
echo ok

printf 'START-23 pagination and empty-result states... '
partners="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners?limit=1&offset=0")"
printf '%s' "$partners" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d.get("items"),list); assert len(d["items"])<=1'
empty="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/partners?q=__start23_no_match_expected__&limit=5&offset=0")"
printf '%s' "$empty" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d.get("items"),list); assert len(d["items"])==0'
echo ok

printf 'START-23 failure-state contract... '
code="$(curl -sS -o "$BODY" -w '%{http_code}' -b "$COOKIE" "$BASE_URL/api/v1/partners/ptr_start23_missing")"
test "$code" = "404"
python3 - "$BODY" <<'PY'
import json,sys
with open(sys.argv[1]) as f:
    data=json.load(f)
assert data, data
PY
echo ok

printf 'START-23 identity and profile surfaces... '
me="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/auth/me")"
printf '%s' "$me" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["system_owner"] is True; assert d["can_manage_users"] is True'
profile="$(curl -fsS -b "$COOKIE" "$BASE_URL/api/v1/profile")"
printf '%s' "$profile" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["system_owner"] is True; assert d["preferred_locale"] in ("en_US","hu_HU")'
echo ok

echo "HIMATE START-23 responsive and functional route-matrix smoke passed"
