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
full_ci_path = workflow_dir / "ci.yml"
full_ci = full_ci_path.read_text(encoding="utf-8")

# The exhaustive workflow must execute every explicit runtime smoke and live
# shell audit. A V2/successor does not exempt the historical smoke contract.
for path in sorted(scripts_dir.glob("smoke_*.sh")):
    invocation = f"sh scripts/{path.name}"
    if invocation not in full_ci:
        failures.append(
            f"{full_ci_path.relative_to(root)}: missing runtime smoke invocation: {invocation}"
        )
for path in sorted(scripts_dir.glob("audit_*.sh")):
    invocation = f"sh scripts/{path.name}"
    if invocation not in full_ci:
        failures.append(
            f"{full_ci_path.relative_to(root)}: missing live audit invocation: {invocation}"
        )

# All static acceptance families are intentionally glob-executed before Go,
# Flutter and Compose so newly added contracts cannot silently fall out of CI.
for pattern in (
    "scripts/audit_smoke_*.py",
    "scripts/audit_start_*.py",
    "scripts/audit_central_*.py",
):
    if pattern not in full_ci:
        failures.append(
            f"{full_ci_path.relative_to(root)}: missing complete static audit glob: {pattern}"
        )

smoke_sources = "\n".join(
    path.read_text(encoding="utf-8") for path in sorted(scripts_dir.glob("smoke_*.sh"))
)
for helper in (
    "evidence_test_helpers.sh",
    "payment_test_helpers.sh",
    "start_23_12_closure_publish.py",
):
    if helper not in smoke_sources:
        failures.append(f"smoke helper is orphaned from the runtime surface: scripts/{helper}")

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
