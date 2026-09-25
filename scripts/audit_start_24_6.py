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

gomod = read("services/go.mod")
fast = read(".github/workflows/ci-fast.yml")
full = read(".github/workflows/ci.yml")
render = read("render.yaml")
vercel = read("vercel.json")

require("go 1.27.0" in gomod and "toolchain go1.27.1" in gomod,
        "Go module is not pinned to the supported Go 1.27.1 toolchain")
require("github.com/jackc/pgx/v5 v5.11.0" in gomod,
        "pgx security baseline is older than v5.11.0")
require("go-version: '1.27.1'" in fast and "go-version: '1.27.1'" in full,
        "CI does not test the exact production Go toolchain")

dockerfiles = sorted((ROOT / "services" / "docker").glob("*.Dockerfile"))
require(len(dockerfiles) >= 20, "expected Go service Dockerfiles are missing")
for path in dockerfiles:
    body = path.read_text()
    require("golang:1.23" not in body, f"unsupported Go 1.23 builder remains in {path.name}")
    require("golang:1.27.1-bookworm" in body, f"{path.name} is not pinned to Go 1.27.1-bookworm")

require("autoDeploy: false" in render,
        "Render production auto-deploy guard is missing")
require('"deploymentEnabled"' in vercel and '"*": false' in vercel,
        "Vercel deployment guard is missing")

# Release metadata must stay synchronized across production services.
require("0.8.32-start-23.11.7" not in render,
        "stale pre-23.12 production version remains in Render Blueprint")
require(render.count("0.8.33-start-23.12") >= 20,
        "Render production services do not share the START-23.12 release identity")

print("START-24.6 supply-chain/deploy acceptance: PASS")
