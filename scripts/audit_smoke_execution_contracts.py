#!/usr/bin/env python3
from pathlib import Path
import re
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
scripts_dir = root / "scripts"
failures = []

for path in sorted(scripts_dir.glob("*.sh")):
    syntax = subprocess.run(
        ["sh", "-n", str(path)],
        capture_output=True,
        text=True,
        check=False,
    )
    if syntax.returncode != 0:
        detail = (syntax.stderr or syntax.stdout).strip()
        failures.append(f"{path.relative_to(root)}: shell syntax error: {detail}")

    for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if re.search(
            r'^\s*exec\s+(?!(?:sh|bash)(?:\s|$))(?:"[^"]*\.sh"|\S*\.sh(?:\s|$))',
            line,
        ):
            failures.append(
                f"{path.relative_to(root)}:{lineno}: direct exec of a .sh file is not portable; "
                "use 'exec sh <script> ...'"
            )

release_smoke = (scripts_dir / "smoke_start_23_11_3i.sh").read_text(encoding="utf-8")
if "smoke_start_23_11_3h.sh" in release_smoke:
    failures.append(
        "START-23.11.3i must be a standalone release-consistency smoke and must not rerun START-23.11.3h"
    )
if "release_consistent" not in release_smoke or "X-Himate-Expected-Version" not in release_smoke:
    failures.append(
        "START-23.11.3i must verify synchronized release health and release-version enforcement"
    )

workflow_dir = root / ".github" / "workflows"
for workflow in sorted(workflow_dir.glob("*.yml")):
    for lineno, line in enumerate(workflow.read_text(encoding="utf-8").splitlines(), start=1):
        stripped = line.strip()
        if "scripts/" in stripped and ".sh" in stripped and re.search(r"(?:run:\s*|^)(?:\./)?scripts/[^\s]+\.sh", stripped):
            failures.append(
                f"{workflow.relative_to(root)}:{lineno}: shell smoke scripts must be invoked through 'sh scripts/...'"
            )

if failures:
    for failure in failures:
        print("FAIL:", failure)
    sys.exit(1)

print("Smoke shell execution contract audit: PASS")
