#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    return (ROOT / path).read_text()

def require(ok: bool, message: str) -> None:
    if not ok:
        print("FAIL:", message)
        sys.exit(1)

phase_scripts = [f"scripts/audit_start_24_{i}.py" for i in range(1, 7)]
for script in phase_scripts:
    require((ROOT / script).is_file(), f"missing START-24 phase audit: {script}")
    subprocess.run([sys.executable, str(ROOT / script)], cwd=ROOT, check=True)

fast = read(".github/workflows/ci-fast.yml")
full = read(".github/workflows/ci.yml")
smoke = read("scripts/smoke_start_24.sh")

for i in range(1, 7):
    name = f"audit_start_24_{i}.py"
    require(name in fast, f"Fast CI does not run {name}")
    require(name in full, f"Full CI does not run {name}")

require("smoke_start_24.sh" in fast, "Fast CI does not run START-24 runtime security smoke")
require("smoke_start_24.sh" in full, "Full CI does not run START-24 runtime security smoke")

for token in [
    "cross-site browser mutation rejection",
    "forged session rejection",
    "authentication throttle and per-client isolation",
    "oversized JSON request remains bounded",
    "unsigned Stripe webhook rejection",
    "post-abuse service health",
]:
    require(token in smoke, f"START-24 runtime smoke is missing: {token}")

require("smoke_start_23_12_closure.sh" in full,
        "START-23.12 production-acceptance regression is no longer part of Full CI")
require("go test -race ./..." in full,
        "Full CI lost Go race regression coverage")

print("START-24.7 cross-phase security closure contract: PASS")
