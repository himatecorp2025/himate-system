#!/usr/bin/env python3
from __future__ import annotations

import re
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
RENDER = ROOT / "render.yaml"
CMD_ROOT = ROOT / "services" / "cmd"

HOST_TARGETS = {
    "PARTNERS_HOSTPORT": "himate-partners",
    "CATALOG_HOSTPORT": "himate-catalog",
    "BILLING_HOSTPORT": "himate-billing",
    "CONTACT_HOSTPORT": "himate-contact",
    "PROVISIONING_HOSTPORT": "himate-provisioning",
    "ENVIRONMENTS_HOSTPORT": "himate-environments",
    "CONNECTOR_HOSTPORT": "himate-connector",
    "HEALTH_HOSTPORT": "himate-health",
    "IMPACT_HOSTPORT": "himate-impact",
    "EVIDENCE_HOSTPORT": "himate-evidence",
    "REPORTS_HOSTPORT": "himate-reports",
    "CMS_HOSTPORT": "himate-cms",
    "STORAGE_HOSTPORT": "himate-storage",
    "BACKUPS_HOSTPORT": "himate-backups",
    "PARTNER_RUNTIME_HOSTPORT": "himate-runtime",
    "NOTIFICATIONS_HOSTPORT": "himate-notifications",
}

SERVICE_TO_CMD = {
    "himate": "gateway",
    **{f"himate-{p.name}": p.name for p in CMD_ROOT.iterdir() if p.is_dir() and p.name != "gateway"},
}

ENV_RE = re.compile(r'(?:os\.Getenv|common\.Env)\("([A-Z0-9_]+_HOSTPORT)"')


def parse_render() -> dict[str, list[tuple[str, str | None, int]]]:
    lines = RENDER.read_text().splitlines()
    services: dict[str, list[tuple[str, str | None, int]]] = {}
    current_service: str | None = None
    in_service = False
    in_env = False

    for i, line in enumerate(lines):
        if re.match(r"^  - type:\s+", line):
            in_service = True
            in_env = False
            current_service = None
            continue
        if not in_service:
            continue

        name = re.match(r"^    name:\s+([^\s]+)\s*$", line)
        if name and current_service is None:
            current_service = name.group(1)
            services.setdefault(current_service, [])
            continue

        if line == "    envVars:":
            in_env = True
            continue

        if in_env:
            key_match = re.match(r"^      - key:\s+([A-Z0-9_]+)\s*$", line)
            if key_match and current_service:
                key = key_match.group(1)
                target = None
                for follow in lines[i + 1 : i + 8]:
                    if re.match(r"^      - key:", follow) or re.match(r"^  - type:", follow):
                        break
                    target_match = re.match(r"^          name:\s+([^\s]+)\s*$", follow)
                    if target_match:
                        target = target_match.group(1)
                services[current_service].append((key, target, i + 1))
            elif re.match(r"^  - type:", line):
                in_env = False

    return services


def code_hostports(cmd: str) -> set[str]:
    folder = CMD_ROOT / cmd
    required: set[str] = set()
    for path in folder.glob("*.go"):
        required.update(ENV_RE.findall(path.read_text()))
    return required


def main() -> None:
    services = parse_render()
    errors: list[str] = []

    for service, entries in services.items():
        keys = [key for key, _, _ in entries]
        duplicates = sorted(key for key, count in Counter(keys).items() if count > 1)
        if duplicates:
            errors.append(f"{service}: duplicate env keys: {', '.join(duplicates)}")

    for service, cmd in sorted(SERVICE_TO_CMD.items()):
        if service not in services:
            continue
        required = code_hostports(cmd)
        configured_entries = {
            key: target for key, target, _ in services[service] if key.endswith("_HOSTPORT")
        }
        configured = set(configured_entries)

        missing = sorted(required - configured)
        extra = sorted(configured - required)
        if missing:
            errors.append(f"{service}: missing code-required Render bindings: {', '.join(missing)}")
        if extra:
            errors.append(f"{service}: unused Render host bindings: {', '.join(extra)}")

        for key in sorted(required & configured):
            expected = HOST_TARGETS.get(key)
            actual = configured_entries[key]
            if expected and actual != expected:
                errors.append(
                    f"{service}: {key} must bind from {expected}, got {actual or 'no fromService target'}"
                )

    if errors:
        raise SystemExit("Render binding audit failed:\n- " + "\n- ".join(errors))

    print("Render Blueprint service bindings match Go service dependencies and contain no duplicate env keys.")


if __name__ == "__main__":
    main()
