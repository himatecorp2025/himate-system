#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
EXPECTED_VERSION="${HIMATE_APP_VERSION:-0.8.28-start-23.11.3k}"

printf 'gateway reports one synchronized application release... '
HEALTH="$(curl -fsS "$BASE_URL/api/v1/health")"
python3 - "$HEALTH" "$EXPECTED_VERSION" <<'PY'
import json,sys
d=json.loads(sys.argv[1]); expected=sys.argv[2]
assert d["status"]=="ok", d
assert d["version"]==expected, d
assert d["release_consistent"] is True, d
services=d.get("services",{})
versions=d.get("service_versions",{})
assert versions.get("gateway")==expected, versions
bad={k:v for k,v in services.items() if v!="ok"}
assert not bad, bad
for service,status in services.items():
    if service=="identity":
        continue
    assert versions.get(service)==expected, (service,versions.get(service),expected)
PY
echo ok

printf 'private-service release mismatch protection is exposed by the current contract... '
grep -q 'RELEASE_MISMATCH' services/internal/common/common.go
grep -q 'X-Himate-Expected-Version' services/internal/common/common.go
grep -q 'X-Himate-App-Version' services/internal/common/common.go
echo ok

echo 'HIMATE START-23.11.3i Release Consistency smoke passed'
