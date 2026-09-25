#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

gateway = read("services/cmd/gateway/main.go")
gateway_tests = read("services/cmd/gateway/main_test.go")
common = read("services/internal/common/common.go")
payments = read("services/cmd/payments/main.go")
cms = read("services/cmd/cms/main.go")
storage = read("services/cmd/storage/main.go")

# Preserve the existing 5-failure / 15-minute authentication policy.
require("state.Failures >= 5" in gateway and "15 * time.Minute" in gateway,
        "authentication throttle policy changed")
require('w.Header().Set("Retry-After", "900")' in gateway,
        "authentication throttle no longer communicates the retry window")

# The in-memory throttle table must not be an unbounded attacker-controlled allocation.
require("loginAttemptMaxEntries = 4096" in gateway,
        "authentication throttle state has no explicit bound")
require("pruneLoginAttemptsLocked" in gateway,
        "authentication throttle state has no pruning routine")
require("len(a.loginAttempts) >= loginAttemptMaxEntries" in gateway,
        "authentication throttle does not enforce its state bound")
require("TestSTART244LoginThrottleStateIsBounded" in gateway_tests,
        "bounded authentication throttle regression test is missing")

# HTTP servers and expensive payload paths remain bounded.
for token in [
    "ReadHeaderTimeout: 10 * time.Second",
    "ReadTimeout:       25 * time.Second",
    "WriteTimeout:      45 * time.Second",
    "IdleTimeout:       90 * time.Second",
]:
    require(token in common, f"HTTP server timeout missing: {token.strip()}")

require("io.LimitReader(r.Body, 1<<20)" in payments,
        "Stripe webhook body no longer has a hard read bound")
require("http.MaxBytesReader(w,r.Body,maxMediaBytes+(1<<20))" in cms.replace(" ", ""),
        "CMS multipart media upload no longer has an HTTP body bound")
require("io.LimitReader(file,maxMediaBytes+1)" in cms.replace(" ", ""),
        "CMS uploaded media stream no longer has an object-size bound")
require("io.LimitReader(r.Body,maxObjectBytes+1)" in storage.replace(" ", ""),
        "storage object upload no longer has a hard object-size bound")

print("START-24.4 abuse/rate-limit/DDoS acceptance: PASS")
