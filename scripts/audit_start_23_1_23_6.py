#!/usr/bin/env python3
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def read(path: str) -> str:
    p = ROOT / path
    if not p.exists():
        raise SystemExit(f"START-23.1-23.6 cross-phase audit failed: missing required file: {path}")
    return p.read_text(encoding="utf-8")

def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit("START-23.1-23.6 cross-phase audit failed: " + message)

def phase_tuple(value: str):
    return tuple(int(part) for part in str(value).split("."))

matrix = json.loads(read("docs/START-23.1_FUNCTIONAL_MATRIX.json"))
ci = read(".github/workflows/ci.yml")
frontend = read("frontend/lib/main.dart")
localization = read("frontend/lib/localization.dart")

require(phase_tuple(matrix.get("completed_through", "0")) >= phase_tuple("23.6"),
        "functional matrix is not completed through START-23.6")

phase_expected = {
    "23.2": 11,
    "23.3": 5,
    "23.4": 7,
    "23.5": 3,
    "23.6": 21,
}
accepted_states = {
    "MUTATION_PROVEN_PROD_UNVERIFIED",
    "CONTROL_HIDDEN_PROD_UNVERIFIED",
}

rows = [x for x in matrix.get("contracts", []) if x.get("target_phase") in phase_expected]
require(len(rows) == sum(phase_expected.values()),
        f"expected {sum(phase_expected.values())} START-23.2-23.6 closure contracts, found {len(rows)}")

for phase, expected in phase_expected.items():
    phase_rows = [x for x in rows if x.get("target_phase") == phase]
    require(len(phase_rows) == expected,
            f"START-{phase} expected {expected} closure contracts, found {len(phase_rows)}")
    for row in phase_rows:
        cid = row.get("id", "<unknown>")
        state = str(row.get("current_state", ""))
        proof = str(row.get("e2e_proof", "")).strip()
        require(state in accepted_states,
                f"{cid} has non-closed state {state!r}")
        require(proof and not proof.lower().startswith("required:"),
                f"{cid} lacks concrete E2E proof")

expected_proof = {
    "23.2": "scripts/smoke_start_23_2.sh",
    "23.3": "scripts/smoke_start_23_3.sh",
    "23.4": "scripts/smoke_start_23_4.sh",
    "23.5": "scripts/smoke_start_23_5.sh",
}
for phase, script in expected_proof.items():
    for row in [x for x in rows if x.get("target_phase") == phase]:
        require(script in str(row.get("e2e_proof", "")),
                f"{row.get('id')} does not retain {script} proof")

for row in [x for x in rows if x.get("target_phase") == "23.6"]:
    if row.get("id") == "AUTH-SSO":
        require(row.get("current_state") == "CONTROL_HIDDEN_PROD_UNVERIFIED",
                "AUTH-SSO must be classified as intentionally hidden while no provider exists")
        require(row.get("localization_state") == "NOT_APPLICABLE",
                "AUTH-SSO localization must be not applicable while the control is absent")
        require("audit_start_23_6.py" in str(row.get("e2e_proof", "")),
                "AUTH-SSO must retain static absence proof")
    else:
        require("scripts/smoke_start_23_6.sh" in str(row.get("e2e_proof", "")),
                f"{row.get('id')} does not retain START-23.6 mutation proof")

# START-23.1 is the contract/inventory reset phase, not a closure-target phase.
for path in (
    "docs/START-23.1_ACCEPTANCE.md",
    "docs/START-23.1_SURFACE_INVENTORY.md",
    "scripts/audit_start_23_1_contract.py",
    "scripts/smoke_start_23_1_mutation_canary.sh",
):
    read(path)

# All later acceptance/static/E2E artifacts must remain present.
for phase in ("23.2", "23.3", "23.4", "23.5", "23.6"):
    read(f"docs/START-{phase}_ACCEPTANCE.md")
    read(f"scripts/audit_start_{phase.replace('.', '_')}.py")
    read(f"scripts/smoke_start_{phase.replace('.', '_')}.sh")

# Unavailable SSO must not silently reappear as a placeholder.
for stale in (
    "Sign in with SSO",
    "ssoPending",
    "onSso",
    "SSO is not configured for this environment yet",
):
    require(stale not in frontend and stale not in localization,
            f"stale SSO placeholder reappeared: {stale!r}")

# CI must continue to run every static audit and every Compose proof.
ci_tokens = (
    "audit_start_23_1_contract.py",
    "audit_start_23_2.py",
    "audit_start_23_3.py",
    "audit_start_23_4.py",
    "audit_start_23_5.py",
    "audit_start_23_6.py",
    "audit_start_23_1_23_6.py",
    "smoke_start_23_1_mutation_canary.sh",
    "smoke_start_23_2.sh",
    "smoke_start_23_3.sh",
    "smoke_start_23_4.sh",
    "smoke_start_23_5.sh",
    "smoke_start_23_6.sh",
)
for token in ci_tokens:
    require(token in ci, f"CI no longer executes or references {token}")

states = {}
for row in rows:
    state = row["current_state"]
    states[state] = states.get(state, 0) + 1

print(
    "START-23.1-23.6 cross-phase audit passed: "
    f"{len(rows)}/{len(rows)} closure contracts closed; states={states}"
)
