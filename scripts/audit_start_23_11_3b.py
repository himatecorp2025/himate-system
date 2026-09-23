#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]

partners = (root / "services/cmd/partners/main.go").read_text(encoding="utf-8")
portal = (root / "services/cmd/gateway/partner_portal.go").read_text(encoding="utf-8")
gateway = (root / "services/cmd/gateway/main.go").read_text(encoding="utf-8")
openapi = (root / "docs/openapi.yaml").read_text(encoding="utf-8")
render = (root / "render.yaml").read_text(encoding="utf-8")

checks = [
    ("partner fixture id", "ptr_himate_test_001" in partners),
    ("partner fixture is live", "'LIVE',FALSE,FALSE" in partners),
    ("partner fixture persistence note", "Persistent manual QA fixture" in partners),
    ("nullable partner category reads are safe", "COALESCE(p.category_id,'')" in partners),
    ("portal fixture user id", 'persistentTestPartnerUserID = "pusr_himate_test_001"' in portal),
    ("portal fixture email", 'persistentTestPartnerEmail = "test.partner@himate.test"' in portal),
    ("portal fixture owner role", "'owner',TRUE,'en_US','UTC'" in portal),
    ("password is stored only as PBKDF2 hash", 'persistentTestPartnerPasswordHash = "pbkdf2-sha256$210000$' in portal),
    ("gateway migration is wired", "persistentTestPartnerMigration()," in gateway),
    ("release version", "version: 0.8.20-start-23.11.3c" in openapi),
    ("render release version", "value: 0.8.20-start-23.11.3c" in render),
]

failures = [label for label, ok in checks if not ok]
if failures:
    for label in failures:
        print("FAIL:", label)
    raise SystemExit(1)

print("START-23.11.3b persistent test partner static audit: PASS")
