#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
gateway = (root / "services/cmd/gateway/main.go").read_text(encoding="utf-8")
portal = (root / "services/cmd/gateway/partner_portal.go").read_text(encoding="utf-8")
tests = (root / "services/cmd/gateway/partner_portal_test.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

checks = [
    ("partners runtime fixture seed", "ON CONFLICT(id) DO NOTHING" in partners and "ptr_himate_test_001" in partners),
    ("gateway runtime portal identity seed", "ON CONFLICT(email) DO NOTHING" in gateway and "persistentTestPartnerPasswordHash" in gateway),
    ("precise registry not-ready error", "PARTNER_REGISTRY_NOT_READY" in portal),
    ("precise registry auth error", "PARTNER_REGISTRY_AUTH_FAILED" in portal),
    ("precise registry timeout error", "PARTNER_REGISTRY_TIMEOUT" in portal),
    ("suspended access remains forbidden", "PARTNER_ACCESS_DISABLED" in portal),
    ("access error classification unit coverage", "TestSTART23113CPartnerAccessErrorClassification" in tests),
    ("release version", "version: 0.8.20-start-23.11.3c" in openapi),
    ("render release version", "value: 0.8.20-start-23.11.3c" in render),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for label in failures:
        print("FAIL:", label)
    raise SystemExit(1)

print("START-23.11.3c production fixture readiness static audit: PASS")
