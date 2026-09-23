#!/usr/bin/env python3
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1]
scripts_dir = root / "scripts"

# Database column names that were valid historically but are no longer part of
# the canonical physical schema. Keep this list small and explicit: these are
# schema-contract migrations that must never reappear in direct smoke SQL.
STALE_COLUMNS = [
    ("identity.partner_users", "role", "role_key"),
]

sql_heredoc = re.compile(r"<<'SQL'\s*\n(.*?)\nSQL", re.DOTALL)
failures = []

for path in sorted(scripts_dir.glob("*.sh")):
    source = path.read_text(encoding="utf-8")
    for block_index, block in enumerate(sql_heredoc.findall(source), start=1):
        # Remove SQL comments and quoted values before scanning identifiers.
        # This prevents values such as 'owner' or prose comments from being
        # mistaken for column references.
        scan = re.sub(r"--.*$", "", block, flags=re.MULTILINE)
        scan = re.sub(r"'(?:''|[^'])*'", "''", scan)
        for table, stale, canonical in STALE_COLUMNS:
            if table.lower() not in scan.lower():
                continue
            if re.search(rf"(?<![A-Za-z0-9_]){re.escape(stale)}(?![A-Za-z0-9_])", scan, flags=re.IGNORECASE):
                failures.append(
                    f"{path.relative_to(root)} SQL block {block_index}: "
                    f"{table}.{stale} is stale; use {canonical}"
                )

partner_portal = (root / "services/cmd/gateway/partner_portal.go").read_text(encoding="utf-8")
if "role_key TEXT NOT NULL DEFAULT 'viewer'" not in partner_portal:
    failures.append("canonical partner_users schema no longer declares role_key as expected")
if re.search(r"CREATE TABLE IF NOT EXISTS identity\.partner_users\([\s\S]*?\n\s*role\s+TEXT\b", partner_portal):
    failures.append("canonical partner_users schema unexpectedly declares legacy role column")

hardening = (scripts_dir / "smoke_start_23_11_3h.sh").read_text(encoding="utf-8")
if "FROM identity.partner_users" not in hardening or "role_key='owner'" not in hardening:
    failures.append("START-23.11.3h physical owner assertion must use identity.partner_users.role_key")

if failures:
    for failure in failures:
        print("FAIL:", failure)
    sys.exit(1)

print("Smoke database schema contract audit: PASS")
