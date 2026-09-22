#!/usr/bin/env sh
set -eu

BASE_URL="${1:-http://127.0.0.1:8080}"
COMPOSE_JSON="$(docker compose config --format json)"
OWNER_EMAIL="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_EMAIL"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_EMAIL=")))')"
OWNER_PASSWORD="$(printf '%s' "$COMPOSE_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); e=d["services"]["gateway"]["environment"]; print(e["HIMATE_BOOTSTRAP_ADMIN_PASSWORD"] if isinstance(e,dict) else next(x.split("=",1)[1] for x in e if x.startswith("HIMATE_BOOTSTRAP_ADMIN_PASSWORD=")))')"

BASE_URL="$BASE_URL" OWNER_EMAIL="$OWNER_EMAIL" OWNER_PASSWORD="$OWNER_PASSWORD" python3 - <<'PY'
import concurrent.futures
import http.cookiejar
import json
import os
import statistics
import sys
import time
import urllib.error
import urllib.request

base = os.environ["BASE_URL"].rstrip("/")
email = os.environ["OWNER_EMAIL"]
password = os.environ["OWNER_PASSWORD"]

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
        raise SystemExit(f"login failed: {response.status}")
cookie_header = "; ".join(f"{c.name}={c.value}" for c in jar)
if not cookie_header:
    raise SystemExit("login returned no session cookie")

def one(path, authenticated):
    headers = {"Accept": "application/json,text/html;q=0.9,*/*;q=0.8"}
    if authenticated:
        headers["Cookie"] = cookie_header
    req = urllib.request.Request(base + path, headers=headers, method="GET")
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=8) as response:
            body = response.read(256)
            status = response.status
    except urllib.error.HTTPError as exc:
        status = exc.code
        body = exc.read(256)
    except Exception as exc:
        return 0, time.perf_counter() - started, repr(exc)
    elapsed = time.perf_counter() - started
    if status != 200:
        return status, elapsed, body.decode("utf-8", "replace")
    return status, elapsed, ""

def percentile(values, q):
    ordered = sorted(values)
    if not ordered:
        return float("inf")
    index = max(0, min(len(ordered)-1, int((len(ordered)-1) * q)))
    return ordered[index]

def scenario(name, path, total, concurrency, authenticated, p95_limit):
    for _ in range(min(10, total)):
        status, elapsed, error = one(path, authenticated)
        if status != 200:
            raise SystemExit(f"{name} warmup failed: status={status} error={error}")

    started = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as pool:
        futures = [pool.submit(one, path, authenticated) for _ in range(total)]
        results = [future.result() for future in futures]
    wall = time.perf_counter() - started

    failures = [r for r in results if r[0] != 200]
    latencies = [r[1] for r in results if r[0] == 200]
    p50 = percentile(latencies, 0.50)
    p95 = percentile(latencies, 0.95)
    maximum = max(latencies) if latencies else float("inf")
    rps = total / wall if wall > 0 else 0.0

    print(
        f"LOAD_AUDIT {name} total={total} concurrency={concurrency} failures={len(failures)} "
        f"p50_ms={p50*1000:.1f} p95_ms={p95*1000:.1f} max_ms={maximum*1000:.1f} rps={rps:.1f}"
    )
    if failures:
        sample = failures[:3]
        raise SystemExit(f"{name}: {len(failures)} failed requests; sample={sample}")
    if p95 > p95_limit:
        raise SystemExit(f"{name}: p95 {p95:.3f}s exceeds {p95_limit:.3f}s audit limit")
    if rps < 5:
        raise SystemExit(f"{name}: throughput {rps:.2f} rps is below minimum audit floor")

scenarios = [
    ("liveness", "/api/v1/live", 500, 25, False, 2.5),
    ("landing", "/", 300, 20, False, 2.5),
    ("partners_page", "/api/v1/partners?limit=50&offset=0", 300, 15, True, 4.0),
    ("health_fanout", "/api/v1/health", 120, 8, True, 6.0),
]
for args in scenarios:
    scenario(*args)

print("HIMATE post-START-22.3 concurrent load audit passed")
PY
