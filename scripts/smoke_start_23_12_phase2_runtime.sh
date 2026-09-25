#!/usr/bin/env sh
set -eu
RUNTIME_URL="${1:-http://127.0.0.1:18081}"
TOKEN="local-development-internal-token-123456789"
VERSION="${HIMATE_APP_VERSION:-0.8.33-start-23.12}"
STAMP="$(date +%s)"
PAYLOAD="$(python3 - "$STAMP" <<'PY'
import json,sys
s=sys.argv[1]
print(json.dumps({
 "partner_id":"ptr_phase2_runtime_"+s,
 "environment":"STAGING",
 "hostname":"phase2-runtime-"+s+".himate.local",
 "release":"phase2-"+s,
 "config":{}
}))
PY
)"

call_runtime() {
  curl -fsS     -H "X-Himate-Internal-Token: $TOKEN"     -H "X-Himate-Expected-Version: $VERSION"     -H 'Content-Type: application/json'     -d "$PAYLOAD"     "$RUNTIME_URL/internal/v1/runtime/deploy"
}

FIRST="$(call_runtime)"
SECOND="$(call_runtime)"
python3 - "$FIRST" "$SECOND" <<'PY'
import json,sys
a,b=map(json.loads,sys.argv[1:3])
assert a["request_key"]==b["request_key"],(a,b)
assert a["provider_deploy_id"]==b["provider_deploy_id"],(a,b)
assert a["provider_deploy_id"],a
print("deployment intent replay reused one provider deployment... ok")
PY

echo 'HIMATE START-23.12 Phase 2 runtime deployment smoke passed'
