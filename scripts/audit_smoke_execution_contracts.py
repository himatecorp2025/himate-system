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

# START-18/19 is an intentionally retained historical runtime contract. The
# legacy asset URL must be satisfied by one Gateway compatibility alias that
# serves the current protected wordmark without restoring the retired asset.
legacy_logo_path = "/art/himate_logo_master_v2.webp"
current_wordmark = "himate_identity_wordmark_2026.webp"
legacy_smoke = (scripts_dir / "smoke_start_18_19.sh").read_text(encoding="utf-8")
gateway_main = (root / "services" / "cmd" / "gateway" / "main.go").read_text(encoding="utf-8")
if legacy_logo_path not in legacy_smoke:
    failures.append("START-18/19 historical logo smoke contract unexpectedly changed")
if gateway_main.count(legacy_logo_path) != 1:
    failures.append("Gateway must expose exactly one historical START-18/19 logo compatibility alias")
alias_start = gateway_main.find(legacy_logo_path)
alias_end = gateway_main.find("\n\t\tmarketingPages :=", alias_start)
alias_block = gateway_main[alias_start:alias_end if alias_end > alias_start else alias_start + 800]
if current_wordmark not in alias_block or "http.ServeFile" not in alias_block:
    failures.append("START-18/19 legacy logo alias does not serve the protected current wordmark")

# Any smoke that inspects the Netscape cookie jar must preserve curl's
# #HttpOnly_ prefix semantics. Dropping every '#' line silently deletes secure
# session cookies and creates a false auth regression.
for path in sorted(scripts_dir.glob("*.sh")):
    source = path.read_text(encoding="utf-8")
    if "himate_session" not in source or "splitlines()" not in source:
        continue
    if 'not line.startswith("#")' in source and "#HttpOnly_" not in source:
        failures.append(
            f"{path.relative_to(root)}: cookie-jar parser discards #HttpOnly_ session records"
        )

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
