#!/usr/bin/env python3
"""START-23.1 functional-contract audit.

This audit intentionally distinguishes route reachability from product functionality.
It fails when a frontend mutation is not registered in the functional contract matrix,
or when the matrix makes an invalid/unsupported acceptance claim.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MATRIX = ROOT / "docs" / "START-23.1_FUNCTIONAL_MATRIX.json"

errors: list[str] = []

if not MATRIX.is_file():
    raise SystemExit("START-23.1 functional matrix is missing")

data = json.loads(MATRIX.read_text())
surfaces = data.get("surfaces", [])
contracts = data.get("contracts", [])
states = set(data.get("states", []))

required_top = {
    "schema_version",
    "start",
    "title",
    "baseline_ref",
    "acceptance_rule",
    "completed_through",
    "states",
    "surfaces",
    "contracts",
}
missing_top = sorted(required_top - set(data))
if missing_top:
    errors.append("matrix missing top-level fields: " + ", ".join(missing_top))

if data.get("schema_version") != 1:
    errors.append("matrix schema_version must be 1")
if data.get("start") != "23.1":
    errors.append("matrix start must be 23.1")

completed_match = re.fullmatch(r"23\.(?:[1-9]|1[0-2])", str(data.get("completed_through", "")))
if not completed_match:
    errors.append("matrix completed_through must be START-23.1 through START-23.12")
    completed_phase = 1
else:
    completed_phase = int(str(data["completed_through"]).split(".", 1)[1])
if len(surfaces) < 50:
    errors.append(f"expected at least 50 inventoried surfaces, got {len(surfaces)}")
if len(contracts) < 80:
    errors.append(f"expected at least 80 functional contracts, got {len(contracts)}")

surface_ids = [str(item.get("id", "")) for item in surfaces]
if len(surface_ids) != len(set(surface_ids)):
    errors.append("surface IDs must be unique")
surface_set = set(surface_ids)

required_surfaces = {
    "auth.admin",
    "dashboard",
    "partners.list",
    "partners.create",
    "partner.modules",
    "modules.registry",
    "finance",
    "impact.metrics",
    "impact.evidence",
    "impact.reports",
    "website.contact",
    "website.cms_pages",
    "website.cms_media",
    "website.design",
    "website.seo",
    "system.health",
    "system.provisioning",
    "system.environments",
    "system.backups",
    "administration.audit",
    "administration.roles",
    "administration.users",
    "profile",
    "notifications",
    "partner_portal.auth",
    "partner_portal.modules",
    "partner_portal.billing",
    "partner_portal.company",
    "partner_portal.users",
    "public.contact",
    "automation.billing",
    "automation.payments",
}
for surface in sorted(required_surfaces - surface_set):
    errors.append(f"required surface missing from inventory: {surface}")

required_contract_fields = {
    "id",
    "surface",
    "ui",
    "kind",
    "frontend_file",
    "source_token",
    "method",
    "api_pattern",
    "backend_service",
    "persistence",
    "audit",
    "localization_state",
    "current_state",
    "target_phase",
    "e2e_proof",
    "notes",
}
ids: list[str] = []
for contract in contracts:
    missing = required_contract_fields - set(contract)
    cid = str(contract.get("id", "<missing-id>"))
    if missing:
        errors.append(f"{cid}: missing fields: {', '.join(sorted(missing))}")
        continue

    ids.append(cid)
    if contract["surface"] not in surface_set:
        errors.append(f"{cid}: unknown surface {contract['surface']}")
    if contract["current_state"] not in states:
        errors.append(f"{cid}: invalid current_state {contract['current_state']}")
    if not re.fullmatch(r"23\.(?:[2-9]|1[0-2])", str(contract["target_phase"])):
        errors.append(f"{cid}: target_phase must be START-23.2 through START-23.12")
    if not str(contract["e2e_proof"]).strip():
        errors.append(f"{cid}: e2e_proof requirement is empty")

    source = ROOT / str(contract["frontend_file"])
    if not source.is_file():
        errors.append(f"{cid}: source file does not exist: {contract['frontend_file']}")
    else:
        source_text = source.read_text()
        token = str(contract["source_token"])
        if token and token not in source_text:
            errors.append(
                f"{cid}: source token not found in {contract['frontend_file']}: {token!r}"
            )

    if contract["kind"] == "mutation":
        if contract["method"] in {"NONE", ""}:
            errors.append(f"{cid}: mutation has no HTTP method")
        if contract["api_pattern"] in {"NONE", ""}:
            errors.append(f"{cid}: mutation has no API pattern")
        if str(contract["backend_service"]).startswith("MISSING"):
            errors.append(f"{cid}: visible mutation must not claim a missing backend; use a blocker kind/state")

if len(ids) != len(set(ids)):
    duplicates = sorted({x for x in ids if ids.count(x) > 1})
    errors.append("contract IDs must be unique: " + ", ".join(duplicates))

contract_by_file: dict[str, list[dict]] = {}
for contract in contracts:
    contract_by_file.setdefault(str(contract.get("frontend_file", "")), []).append(contract)

# Every Flutter mutation invocation must be represented in the matrix.
mutation_call = re.compile(
    r"\b(?:widget\.)?api\.(?:post|put|patch|delete|multipart)\s*\(",
    re.MULTILINE,
)
unregistered: list[str] = []
for path in sorted((ROOT / "frontend" / "lib").glob("*.dart")):
    rel = path.relative_to(ROOT).as_posix()
    source = path.read_text()
    file_contracts = [
        item
        for item in contract_by_file.get(rel, [])
        if item.get("kind") == "mutation"
    ]
    for match in mutation_call.finditer(source):
        start = max(0, match.start() - 80)
        end = min(len(source), match.start() + 900)
        window = source[start:end]
        if any(str(item.get("source_token", "")) in window for item in file_contracts):
            continue
        line = source.count("\n", 0, match.start()) + 1
        compact = " ".join(window.split())[:300]
        unregistered.append(f"{rel}:{line}: {compact}")

if unregistered:
    errors.append(
        "unregistered Flutter mutation calls:\n  - " + "\n  - ".join(unregistered)
    )

# Public-site write path must also be inventoried.
public_contact = ROOT / "frontend" / "web" / "contact.html"
if public_contact.is_file() and "fetch('/api/v1/public/contact'" in public_contact.read_text():
    if not any(c.get("id") == "PUBLIC-CONTACT" for c in contracts):
        errors.append("public contact mutation is not represented in the functional matrix")

# Known product blockers may not disappear from the matrix until a later phase
# explicitly replaces the corresponding implementation and contract state.
required_blockers = {
    "AUTH-FORGOT": "PLACEHOLDER",
    "AUTH-SSO": "PLACEHOLDER",
    "DASH-REVENUE": "MOCK_OR_EMPTY",
    "DASH-PEOPLE": "MOCK_OR_EMPTY",
    "DASH-CHART": "MOCK_DATA",
    "DASH-ACTIVITY": "MOCK_DATA",
    "DASH-SEARCH": "PLACEHOLDER",
    "CMS-ARBITRARY-SECTION": "PARTIAL_PRODUCT",
    "DESIGN-PREVIEW": "PARTIAL_PRODUCT",
    "BILLING-SCHEDULER": "MISSING_PRODUCTION_AUTOMATION",
    "PAYMENT-AUTOPAY": "MISSING_BACKEND_INTEGRATION",
    "MODULE-CANCELLATION-CONSISTENCY": "SEMANTIC_GAP",
    "DYNAMIC-BILINGUAL-MODEL": "MISSING_DATA_MODEL",
}
by_id = {str(c.get("id")): c for c in contracts}
for cid, expected in required_blockers.items():
    item = by_id.get(cid)
    if item is None:
        errors.append(f"known blocker missing from matrix: {cid}")
        continue
    target_phase = int(str(item.get("target_phase", "23.12")).split(".", 1)[1])
    current = item.get("current_state")
    if target_phase > completed_phase:
        if current != expected:
            errors.append(
                f"{cid}: blocker must remain {expected} until target phase {item.get('target_phase')} closes it; "
                f"got {current}"
            )
    elif current == expected:
        errors.append(
            f"{cid}: target phase {item.get('target_phase')} is already completed but blocker still remains {expected}"
        )

# Production-proven claims need a concrete proof reference.
for contract in contracts:
    if contract.get("current_state") != "PROD_PROVEN":
        continue
    proof = str(contract.get("e2e_proof", "")).lower()
    if not any(term in proof for term in ("script", "production", "render", "start-21")):
        errors.append(f"{contract['id']}: PROD_PROVEN lacks a concrete proof reference")

# START-24 must remain blocked in the 23.1 acceptance document.
acceptance = (ROOT / "docs" / "START-23.1_ACCEPTANCE.md").read_text()
if "START-24 is blocked until START-23.12 passes" not in acceptance:
    errors.append("START-23.1 acceptance must explicitly block START-24 until START-23.12")
if "A GET-only smoke is therefore **route evidence**, not functional mutation evidence." not in acceptance:
    errors.append("START-23.1 acceptance must distinguish GET reachability from mutation proof")

if errors:
    raise SystemExit("START-23.1 functional-contract audit failed:\n- " + "\n- ".join(errors))

mutation_count = sum(1 for item in contracts if item.get("kind") == "mutation")
closed_states = {
    "SOURCE_COMPLETE_PROD_UNVERIFIED",
    "PROD_PROVEN",
    "MUTATION_PROVEN_PROD_UNVERIFIED",
    "CONTROL_HIDDEN_PROD_UNVERIFIED",
}
blocker_count = sum(
    1
    for item in contracts
    if item.get("current_state") not in closed_states
)
print(
    "START-23.1 functional-contract audit passed: "
    f"{len(surfaces)} surfaces, {len(contracts)} contracts, "
    f"{mutation_count} mutations, {blocker_count} explicit blockers/gaps."
)
