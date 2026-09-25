#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

PARTNER_ID="$(BASE_URL="$BASE_URL" OWNER_EMAIL="$OWNER_EMAIL" OWNER_PASSWORD="$OWNER_PASSWORD" python3 - <<'PY'
import http.cookiejar
import json
import os
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone

base = os.environ["BASE_URL"].rstrip("/")
email = os.environ["OWNER_EMAIL"]
password = os.environ["OWNER_PASSWORD"]

try:
    urllib.request.urlopen(base + "/api/v1/archives", timeout=5)
    raise SystemExit("unauthenticated Compliance Archives unexpectedly returned success")
except urllib.error.HTTPError as exc:
    if exc.code != 401:
        raise SystemExit(f"unauthenticated archive status={exc.code}, expected 401")

jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))

def request(path, method="GET", payload=None, headers=None):
    body = None if payload is None else json.dumps(payload).encode()
    h = {"Accept": "application/json"}
    if body is not None:
        h["Content-Type"] = "application/json"
    if headers:
        h.update(headers)
    req = urllib.request.Request(base + path, data=body, headers=h, method=method)
    try:
        with opener.open(req, timeout=12) as response:
            raw = response.read()
            return response.status, json.loads(raw or b"{}")
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        try:
            data = json.loads(raw or b"{}")
        except Exception:
            data = {"raw": raw.decode("utf-8", "replace")}
        return exc.code, data

status, _ = request("/api/v1/auth/login", "POST", {"email": email, "password": password, "remember": False})
if status != 200:
    raise SystemExit(f"admin login failed: {status}")

stamp = str(int(time.time() * 1000))
status, created = request("/api/v1/partners", "POST", {
    "display_name": "Phase 5 Compliance " + stamp,
    "legal_name": "Phase 5 Compliance Test LLC",
    "brand_name": "Phase 5 Compliance",
    "category_id": "cat_006",
    "lifecycle": "PROSPECT",
    "country": "United States",
    "contact_name": "Phase Five",
    "contact_email": f"phase5-{stamp}@example.test",
    "onboarding_request_id": "phase5-archive-" + stamp,
})
if status not in (200, 201):
    raise SystemExit(f"partner creation failed: {status} {created}")
partner_id = str(created.get("id") or "")
if not partner_id.startswith("ptr_"):
    raise SystemExit(f"invalid partner id: {partner_id!r}")

status, archived = request("/api/v1/partners/" + partner_id, "PATCH", {
    "lifecycle": "ARCHIVED",
    "reason": "START-23.12 Phase 5 immutable compliance acceptance",
})
if status != 200 or archived.get("lifecycle") != "ARCHIVED":
    raise SystemExit(f"partner archive transition failed: {status} {archived}")

status, detail = request("/api/v1/archives/" + partner_id)
if status != 200:
    raise SystemExit(f"Compliance Archive detail failed: {status} {detail}")
if detail.get("integrity_status") != "VERIFIED" or detail.get("read_only") is not True:
    raise SystemExit(f"Compliance Archive integrity/read-only contract failed: {detail}")
if int(detail.get("retention_years", 0)) != 7:
    raise SystemExit(f"unexpected archive retention: {detail.get('retention_years')}")

archived_at = datetime.fromisoformat(str(detail["archived_at"]).replace("Z", "+00:00")).astimezone(timezone.utc)
retain_until = datetime.fromisoformat(str(detail["retain_until"]).replace("Z", "+00:00")).astimezone(timezone.utc)
if (retain_until - archived_at).days < 365 * 7:
    raise SystemExit("Compliance Archive retention is shorter than seven years")

payload = detail.get("payload") or {}
excluded = set(payload.get("excluded_sensitive_domains") or [])
required_exclusions = {
    "identity.users", "identity.partner_users", "identity.sessions",
    "identity.mfa_challenges", "identity.platform_secrets", "payments.partner_profiles",
}
if not required_exclusions.issubset(excluded):
    raise SystemExit("Compliance Archive credential exclusions are incomplete")

status, listing = request("/api/v1/archives?q=" + urllib.parse.quote(partner_id))
if status != 200 or not any(item.get("partner_id") == partner_id for item in listing.get("items", [])):
    raise SystemExit(f"archived partner is not discoverable: {status} {listing}")

status, _ = request("/api/v1/archives/" + partner_id, "POST", {})
if status != 405:
    raise SystemExit(f"originless archive mutation status={status}, expected 405")

status, _ = request(
    "/api/v1/archives/" + partner_id,
    "POST",
    {},
    {"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"},
)
if status != 403:
    raise SystemExit(f"cross-site archive mutation status={status}, expected Phase 4 rejection 403")

print(partner_id)
PY
)"

case "$PARTNER_ID" in
  ptr_*) ;;
  *) echo "Invalid archive partner id: $PARTNER_ID" >&2; exit 1 ;;
esac

if docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1   -c "UPDATE compliance.partner_archives SET archive_reason='tamper' WHERE partner_id='$PARTNER_ID';" >/tmp/phase5_archive_mutation.log 2>&1; then
  echo "FAIL: Compliance Archive UPDATE unexpectedly succeeded" >&2
  cat /tmp/phase5_archive_mutation.log >&2
  exit 1
fi

docker compose exec -T postgres psql -U himate -d himate -v ON_ERROR_STOP=1 <<'SQL' >/dev/null
INSERT INTO partners.partners(
  id,slug,display_name,legal_name,brand_name,category_id,lifecycle,country,notes
)
SELECT
  'ptr_phase5_load_' || lpad(n::text,3,'0'),
  'phase5-load-' || lpad(n::text,3,'0'),
  'Phase 5 Load Tenant ' || lpad(n::text,3,'0'),
  'Phase 5 Load Tenant ' || lpad(n::text,3,'0') || ' LLC',
  'Phase 5 Load Tenant ' || lpad(n::text,3,'0'),
  'cat_006','LIVE','United States','Synthetic 100-tenant START-23.12 Phase 5 capacity fixture'
FROM generate_series(1,100) AS n
ON CONFLICT(id) DO NOTHING;
SQL

BASE_URL="$BASE_URL" OWNER_EMAIL="$OWNER_EMAIL" OWNER_PASSWORD="$OWNER_PASSWORD" ARCHIVE_PARTNER_ID="$PARTNER_ID" python3 - <<'PY'
import concurrent.futures
import http.cookiejar
import json
import os
import statistics
import time
import urllib.error
import urllib.request

base = os.environ["BASE_URL"].rstrip("/")
email = os.environ["OWNER_EMAIL"]
password = os.environ["OWNER_PASSWORD"]
archive_partner = os.environ["ARCHIVE_PARTNER_ID"]

jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
login = urllib.request.Request(
    base + "/api/v1/auth/login",
    data=json.dumps({"email": email, "password": password, "remember": False}).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
with opener.open(login, timeout=10) as response:
    if response.status != 200:
        raise SystemExit(f"load login failed: {response.status}")
cookie = "; ".join(f"{c.name}={c.value}" for c in jar)
if not cookie:
    raise SystemExit("load login returned no session cookie")

def one(path):
    req = urllib.request.Request(
        base + path,
        headers={"Accept": "application/json", "Cookie": cookie},
        method="GET",
    )
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            response.read(256)
            status = response.status
    except urllib.error.HTTPError as exc:
        status = exc.code
    except Exception:
        return 0, time.perf_counter() - started
    return status, time.perf_counter() - started

def percentile(values, q):
    values = sorted(values)
    if not values:
        return float("inf")
    return values[min(len(values)-1, int((len(values)-1)*q))]

def scenario(name, paths, concurrency, p95_limit, min_rps):
    for path in paths[:min(10, len(paths))]:
        status, _ = one(path)
        if status != 200:
            raise SystemExit(f"{name} warmup failed: {status} {path}")
    started = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as pool:
        results = list(pool.map(one, paths))
    wall = time.perf_counter() - started
    failures = [status for status, _ in results if status != 200]
    latencies = [elapsed for status, elapsed in results if status == 200]
    p50 = percentile(latencies, .50)
    p95 = percentile(latencies, .95)
    rps = len(paths) / wall if wall > 0 else 0
    print(
        f"PHASE5_LOAD {name} requests={len(paths)} concurrency={concurrency} "
        f"failures={len(failures)} p50_ms={p50*1000:.1f} p95_ms={p95*1000:.1f} rps={rps:.1f}"
    )
    if failures:
        raise SystemExit(f"{name}: {len(failures)} failed requests")
    if p95 > p95_limit:
        raise SystemExit(f"{name}: p95 {p95:.3f}s > {p95_limit:.3f}s")
    if rps < min_rps:
        raise SystemExit(f"{name}: throughput {rps:.2f} < {min_rps:.2f} rps")

tenant_ids = [f"ptr_phase5_load_{n:03d}" for n in range(1, 101)]
detail_paths = [f"/api/v1/partners/{tenant_id}" for _ in range(4) for tenant_id in tenant_ids]
portfolio_paths = ["/api/v1/partners?limit=100&offset=0&include_archived=true&include_stats=false"] * 160
archive_paths = [f"/api/v1/archives/{archive_partner}"] * 120

scenario("100_tenant_detail", detail_paths, 32, 4.0, 8.0)
scenario("100_tenant_portfolio", portfolio_paths, 20, 5.0, 5.0)
scenario("compliance_archive_read", archive_paths, 16, 4.0, 5.0)

for tenant_id in tenant_ids:
    status, _ = one(f"/api/v1/partners/{tenant_id}")
    if status != 200:
        raise SystemExit(f"100-tenant verification failed for {tenant_id}: {status}")

print("START-23.12 Phase 5 100-tenant capacity acceptance: PASS")
PY

echo "START-23.12 Phase 5 compliance, Phase 4 security inheritance and 100-tenant load smoke: PASS"
