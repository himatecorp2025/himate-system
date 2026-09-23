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

release_wrapper = (scripts_dir / "smoke_start_23_11_3i.sh").read_text(encoding="utf-8")
required_delegate = 'exec sh "$SCRIPT_DIR/smoke_start_23_11_3h.sh"'
if required_delegate not in release_wrapper:
    failures.append(
        "START-23.11.3i wrapper must delegate through 'exec sh' so file mode cannot break CI"
    )

if failures:
    for failure in failures:
        print("FAIL:", failure)
    sys.exit(1)

print("Smoke shell execution contract audit: PASS")
